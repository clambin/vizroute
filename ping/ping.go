// Package ping sends and receives icmp echo request/reply packets over a UDP socket.  Both IPv4 and IPv6 are supported.
//
// A process using this package can only have one Socket instance.
package ping

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	timeoutInterval = 2 * time.Second
	readTimeout     = 5 * time.Second
)

var (
	errIncorrectID = errors.New("packet ignored: incorrect ID")
)

// The nextID variable is used to generate unique IDs for icmp packets sent by each Socket instance.
// This allows us to run multiple Socket instances in parallel without interfering with each other.
var nextID = uint32(os.Getpid())

// getNextID returns the next unique ID for icmp packets sent by the current Socket instance.
func getNextID() uint16 {
	id := atomic.AddUint32(&nextID, 1)
	return uint16(id & 0xffff)
}

// Transport represents the transport protocol (IPv4 or IPv6) used by the Socket.
type Transport int

const (
	IPv4 Transport = 0x01
	IPv6 Transport = 0x02
)

func (tp Transport) String() string {
	switch tp {
	case IPv4:
		return "ipv4"
	case IPv6:
		return "ipv6"
	default:
		return "unknown"
	}
}

func (tp Transport) Protocol() int {
	switch tp {
	case IPv4:
		return 1
	case IPv6:
		return 58
	default:
		return -1
	}
}

// SequenceNumber represents the sequence number of an icmp packet.
type SequenceNumber uint16

var _ slog.LogValuer = Response{}

// Response represents an icmp packet received by the Socket.
type Response struct {
	From         net.IP
	Request      Request
	ResponseType ResponseType
	Latency      time.Duration
}

func (r Response) LogValue() slog.Value {
	attrs := []slog.Attr{slog.String("type", r.ResponseType.String())}
	if r.ResponseType != ResponseTimeout {
		attrs = append(attrs,
			slog.String("from", r.From.String()),
			slog.String("target", r.Request.Target.String()),
			slog.String("seq", fmt.Sprintf("%d", r.Request.Seq)),
			slog.String("ttl", fmt.Sprintf("%d", r.Request.TTL)),
		)
	}
	return slog.GroupValue(attrs...)
}

// Request represents an icmp packet sent by the Socket.
type Request struct {
	TimeSent time.Time
	Target   net.IP
	Seq      SequenceNumber
	TTL      uint8
}

const (
	ResponseEchoReply ResponseType = iota
	ResponseTimeExceeded
	ResponseTimeout
)

type ResponseType int

func (rt ResponseType) String() string {
	switch rt {
	case ResponseEchoReply:
		return "echo reply"
	case ResponseTimeExceeded:
		return "time exceeded"
	case ResponseTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

type Socket struct {
	v4     *icmp.PacketConn
	v6     *icmp.PacketConn
	q      *responseQueue
	logger *slog.Logger

	outstandingRequests map[SequenceNumber]Request
	Timeout             time.Duration
	lock                sync.Mutex
	id                  uint16
}

// New creates a new Socket instance.
// The provided Transport parameter specifies which transport protocols to use (IPv4 or IPv6).
// If the parameter is 0, both IPv4 and IPv6 will be used.
// The provided logger is used to log events related to the socket.
func New(tp Transport, l *slog.Logger) (*Socket, error) {
	s := Socket{
		q:                   newResponseQueue(),
		logger:              l,
		Timeout:             readTimeout,
		id:                  getNextID(),
		outstandingRequests: make(map[SequenceNumber]Request),
	}
	if tp == 0 {
		tp = IPv4 | IPv6
	}
	var err, errs error
	if tp&IPv4 != 0 {
		if s.v4, err = icmp.ListenPacket("udp4", "0.0.0.0"); err != nil {
			s.v4 = nil
			errs = errors.Join(errs, err)
		}
	}
	if tp&IPv6 != 0 {
		if s.v6, err = icmp.ListenPacket("udp6", "::"); err != nil {
			s.v6 = nil
			errs = errors.Join(errs, err)
		}
	}
	return &s, errs
}

// Resolve resolves the provided host to an IP address and returns it.
// Resolve returns an error if the host does not have a valid IP address of a type supported by the socket
// (e.g., if the socket only supports IPv6, but the host doesn't have an IPv4 address).
func (s *Socket) Resolve(host string) (net.IP, error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve %s: %w", host, err)
	}

	s.logger.Debug("resolved host", "host", host, "ips", len(ips))

	for _, ip := range ips {
		tp := getTransport(ip)
		s.logger.Debug("examining IP", "ip", ip, "tp", int(tp), "tps", tp, "s.v4", s.v4 != nil, "s.v6", s.v6 != nil)
		if (tp == IPv6 && s.v6 != nil) || tp == IPv4 && s.v4 != nil {
			s.logger.Debug("resolved IP", "ip", ip, "tp", tp)
			return ip, nil
		}
	}
	s.logger.Debug("no matching IP found")
	return nil, fmt.Errorf("no valid IP support for %s", host)
}

var echoRequestTypes = map[Transport]icmp.Type{
	IPv4: ipv4.ICMPTypeEcho,
	IPv6: ipv6.ICMPTypeEchoRequest,
}

// Send creates an icmp packet with the provided seq, ttl and payload and sends it to the specified target.
func (s *Socket) Send(target net.IP, seq SequenceNumber, ttl uint8, payload []byte) error {
	// we're setting socket options, so only send one packet at a time
	s.lock.Lock()
	defer s.lock.Unlock()

	// get the right socket for the target's IP type (ipv4 or ipv6)
	var socket *icmp.PacketConn
	tp := getTransport(target)
	switch tp {
	case IPv4:
		//s.logger.Debug("selecting IPv4 socket")
		socket = s.v4
	case IPv6:
		//s.logger.Debug("selecting IPv6 socket")
		socket = s.v6
	default:
		return fmt.Errorf("socket does not support %s", tp)
	}

	// create the ICMP echo Request message
	msg := icmp.Message{
		Type: echoRequestTypes[tp],
		Code: 0,
		Body: &icmp.Echo{
			ID:   int(s.id),
			Seq:  int(seq),
			Data: payload,
		},
	}
	data, _ := msg.Marshal(nil)
	// if ttl is specified, set it on the socket
	if ttl != 0 {
		if err := s.setTTL(ttl); err != nil {
			return fmt.Errorf("icmp socket failed to set ttl: %w", err)
		}
	}
	// send the packet
	s.logger.Debug("sending packet", "addr", target, "ttl", ttl)
	if _, err := socket.WriteTo(data, &net.UDPAddr{IP: target}); err != nil {
		return err
	}

	// mark an outstanding packet for seq & time sent
	s.outstandingRequests[seq] = Request{
		Target:   target,
		TTL:      ttl,
		Seq:      seq,
		TimeSent: time.Now(),
	}
	return nil
}

func (s *Socket) Read(ctx context.Context) (Response, error) {
	subCtx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()

	r, err := s.q.popWait(subCtx)
	if err != nil {
		return Response{}, errors.New("timeout waiting for response")
	}
	return r, nil
}

// Serve listens for icmp packets on the socket and dispatches them to the appropriate handler.
// It's the responsibility of the caller to call Serve before sending or receiving packets.
// Serve blocks until the context is canceled.
func (s *Socket) Serve(ctx context.Context) {
	ch := make(chan Response)
	if s.v4 != nil {
		go s.readPackets(ctx, s.v4, IPv4, ch)
	}
	if s.v6 != nil {
		go s.readPackets(ctx, s.v6, IPv6, ch)
	}
	timeoutTicker := time.NewTicker(timeoutInterval)
	defer timeoutTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeoutTicker.C:
			s.timeout()
		case resp := <-ch:
			s.lock.Lock()
			// process the response:
			// if not an outstanding packet, drop it
			if _, ok := s.outstandingRequests[resp.Request.Seq]; !ok {
				s.logger.Debug("ignoring packet", "seq", resp.Request.Seq)
			} else {
				// queue for delivery by Receive and remove the outstanding packet
				s.q.push(resp)
			}
			s.lock.Unlock()
		}
	}
}

func (s *Socket) readPackets(ctx context.Context, socket *icmp.PacketConn, tp Transport, ch chan Response) {
	logger := s.logger.With("transport", tp)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			response, err := s.readPacket(socket, tp)
			if errors.Is(err, errIncorrectID) {
				logger.Debug("ignoring received packet", "err", err)
				continue
			}
			if err != nil {
				logger.Warn("failed to read packet", "err", err)
				break
			}
			ch <- response
		}
	}
}

func (s *Socket) readPacket(socket *icmp.PacketConn, tp Transport) (Response, error) {
	if err := socket.SetReadDeadline(time.Now().Add(s.Timeout)); err != nil {
		return Response{}, fmt.Errorf("failed to set deadline: %w", err)
	}
	const maxPacketSize = 1500
	buff := make([]byte, maxPacketSize)
	n, from, err := socket.ReadFrom(buff)
	if err != nil {
		return Response{}, fmt.Errorf("read: %w", err)
	}

	var msgID int
	var respType ResponseType
	var seq SequenceNumber

	resp, err := icmp.ParseMessage(tp.Protocol(), buff[:n])
	if err != nil {
		return Response{}, fmt.Errorf("parse: %w", err)
	}
	switch body := resp.Body.(type) {
	case *icmp.Echo:
		respType = ResponseEchoReply
		msgID = body.ID
		seq = SequenceNumber(body.Seq)
	case *icmp.TimeExceeded:
		respType = ResponseTimeExceeded
		msgID, seq, err = parseTimeExceeded(body.Data, from.(*net.UDPAddr).IP)
		if err != nil {
			return Response{}, fmt.Errorf("parse time exceeded payload: %w", err)
		}
	default:
		return Response{}, fmt.Errorf("unknown response type: %T", body)
	}

	// if the packet is not for our id, drop it
	if msgID != int(s.id) {
		return Response{}, errIncorrectID
	}

	// find back the original request
	s.lock.Lock()
	defer s.lock.Unlock()
	req, ok := s.outstandingRequests[seq]
	if !ok {
		return Response{}, fmt.Errorf("no request found for seq %d", seq)
	}

	return Response{
		ResponseType: respType,
		From:         from.(*net.UDPAddr).IP,
		Latency:      time.Since(s.outstandingRequests[seq].TimeSent),
		Request:      req,
	}, nil
}

// timeout removes any outstanding packets that have timed out and queue a timeout response for each of them.
func (s *Socket) timeout() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for seq, req := range s.outstandingRequests {
		if time.Since(req.TimeSent) > s.Timeout {
			s.logger.Debug("timeout expired", "seq", seq)
			s.q.push(Response{
				ResponseType: ResponseTimeout,
			})
			delete(s.outstandingRequests, seq)
		}
	}
}

// setTTL sets the ttl on the socket to the provided value.
func (s *Socket) setTTL(ttl uint8) (err error) {
	if s.v4 != nil {
		err = s.v4.IPv4PacketConn().SetTTL(int(ttl))
	}
	if s.v6 != nil {
		err = errors.Join(err, s.v6.IPv6PacketConn().SetHopLimit(int(ttl)))
	}
	return err
}

// getTransport returns the Transport for the provided IP address (ipv4 or ipv6).
func getTransport(ip net.IP) Transport {
	if ip.To4() != nil {
		return IPv4
	}
	if ip.To16() != nil {
		return IPv6
	}
	return 0
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////

type responseQueue struct {
	notEmpty sync.Cond
	queue    []Response
	lock     sync.Mutex
}

func newResponseQueue() *responseQueue {
	q := &responseQueue{}
	q.notEmpty.L = &q.lock
	return q
}

func (q *responseQueue) push(r Response) {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.queue = append(q.queue, r)
	q.notEmpty.Broadcast()
}

func (q *responseQueue) pop() (Response, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if len(q.queue) == 0 {
		return Response{}, false
	}
	r := q.queue[0]
	q.queue = q.queue[1:]
	return r, true
}

func (q *responseQueue) popWait(ctx context.Context) (Response, error) {
	for {
		if resp, ok := q.pop(); ok {
			return resp, nil
		}
		notEmpty := make(chan struct{})
		go func() {
			q.lock.Lock()
			q.notEmpty.Wait()
			q.lock.Unlock()
			notEmpty <- struct{}{}
		}()
		select {
		case <-ctx.Done():
			return Response{}, ctx.Err()
		case <-notEmpty:
		}
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

// parseTimeExceeded extracts Echo ID and Seq from inner ICMP packet
// Supports both IPv4 and IPv6 TimeExceeded messages
func parseTimeExceeded(data []byte, src net.IP) (id int, seq SequenceNumber, err error) {
	if src.To4() != nil {
		return parseTimeExceededV4(data)
	}
	return parseTimeExceededV6(data)
}

func parseTimeExceededV4(data []byte) (id int, seq SequenceNumber, err error) {
	if len(data) < ipv4.HeaderLen+8 {
		return 0, 0, errors.New("IPv4 payload too short")
	}
	hlen := int(data[0]&0x0f) * 4
	if len(data) < hlen+8 {
		return 0, 0, errors.New("IPv4 inner payload too short")
	}
	inner := data[hlen : hlen+8]
	id = int(binary.BigEndian.Uint16(inner[4:6]))
	seq = SequenceNumber(binary.BigEndian.Uint16(inner[6:8]))
	return id, seq, nil
}

func parseTimeExceededV6(data []byte) (id int, seq SequenceNumber, err error) {
	if len(data) < ipv6.HeaderLen {
		return 0, 0, errors.New("IPv6 payload too short")
	}
	inner := data[ipv6.HeaderLen:]
	m, err := icmp.ParseMessage(IPv6.Protocol(), inner)
	if err != nil {
		return 0, 0, err
	}
	switch b := m.Body.(type) {
	case *icmp.Echo:
		return b.ID, SequenceNumber(b.Seq), nil
	default:
		if len(inner) >= 8 {
			id = int(binary.BigEndian.Uint16(inner[4:6]))
			seq = SequenceNumber(binary.BigEndian.Uint16(inner[6:8]))
			return id, seq, nil
		}
		return 0, 0, errors.New("inner ICMPv6 not Echo and too short")
	}
}
