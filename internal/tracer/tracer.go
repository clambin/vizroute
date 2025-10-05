package tracer

import (
	"context"
	"log/slog"
	"maps"
	"math/rand"
	"net"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/clambin/vizroute/ping"
)

// Socket interface for sending/receiving ICMP packets
type Socket interface {
	Resolve(host string) (net.IP, error)
	Read(ctx context.Context) (ping.Response, error)
	Send(ip net.IP, seq ping.SequenceNumber, ttl uint8, payload []byte) error
}

var _ Socket = (*ping.Socket)(nil)

// Tracer manages the traceroute and continuous pinging
type Tracer struct {
	sock   Socket
	logger *slog.Logger
	hops   map[int]*HopStats // keyed by TTL
	mu     sync.Mutex
}

// NewTracer creates a reusable Tracer
func NewTracer(sock Socket, logger *slog.Logger) *Tracer {
	return &Tracer{
		sock:   sock,
		logger: logger,
		hops:   make(map[int]*HopStats),
	}
}

// Hops returns a snapshot of hop stats in TTL order
func (t *Tracer) Hops() []*HopStats {
	t.mu.Lock()
	defer t.mu.Unlock()

	hops := slices.Collect(maps.Values(t.hops))
	sort.Slice(hops, func(i, j int) bool { return hops[i].TTL < hops[j].TTL })
	return hops
}

func (t *Tracer) lastHop() *HopStats {
	hops := t.Hops()
	if len(hops) == 0 {
		return nil
	}
	return hops[len(hops)-1]
}

// ResetStats resets all hop stats
func (t *Tracer) ResetStats() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, h := range t.hops {
		h.Reset()
	}
}

// Run starts the traceroute to the target host
func (t *Tracer) Run(ctx context.Context, target string, maxHops int) error {
	// Resolve the target
	dest, err := t.sock.Resolve(target)
	if err != nil {
		return err
	}

	// Reset hops for reuse
	t.mu.Lock()
	t.hops = make(map[int]*HopStats)
	t.mu.Unlock()

	// Start reader
	go func() {
		for {
			resp, err := t.sock.Read(ctx)
			if err != nil {
				return
			}
			t.handleResponse(ctx, resp)
		}
	}()

	// send probes for each TTL until we reach the target
	for ttl := 1; ttl <= maxHops; ttl++ {
		// if we've reached the target, stop sending more probes
		if lastHop := t.lastHop(); lastHop != nil && lastHop.IP().Equal(dest) {
			t.logger.Info("reached target", "dest", dest, "ttl", ttl)
			break
		}
		// send the probe
		if err := t.pingTarget(dest, ttl); err != nil {
			t.logger.Error("failed to send probe", "err", err)
			return err
		}
		// wait a bit allow the response to be processed so we can check if we've reached the target
		time.Sleep(time.Second)
	}

	<-ctx.Done()
	return nil
}

// pingTarget sends a single ICMP probe for the given TTL
func (t *Tracer) pingTarget(dest net.IP, ttl int) error {
	id := rand.Uint32() & 0xffff
	seq := 1

	t.logger.Debug("sending probe", "dest", dest, "ttl", ttl, "id", id, "seq", seq)

	// create a new hop stats object for this hop, but don't add the address yet:
	// this will be added when the response is received.
	h := HopStats{
		TTL:       uint8(ttl),
		sentTimes: make(map[int]time.Time),
	}
	h.recordSend(seq)

	t.mu.Lock()
	t.hops[ttl] = &h
	t.mu.Unlock()

	return t.sock.Send(dest, ping.SequenceNumber(seq), uint8(ttl), []byte("probe"))
}

// handleResponse processes an ICMP response and updates hop stats
func (t *Tracer) handleResponse(ctx context.Context, resp ping.Response) {
	t.logger.Debug("packet received", "packet", resp)

	t.mu.Lock()
	defer t.mu.Unlock()

	var hop *HopStats
	var ok bool
	switch resp.ResponseType {
	case ping.ResponseTimeExceeded:
		// response to an initial probe with too low ttl. use request TTL to find the hop
		if hop, ok = t.hops[int(resp.Request.TTL)]; ok {
			hop.recordAddr(resp.From)
		}
	case ping.ResponseEchoReply:
		// response from either the target or a found hop. use request IP to find the hop
		if hop, ok = t.hops[int(resp.Request.TTL)]; ok {
			// found it by looking up the TTL.  it must be the response to the probe
			hop.recordAddr(resp.From)
		} else {
			// just a normal ping response. find the hop by IP
			for _, h := range t.hops {
				if h.IP().Equal(resp.From) {
					ok = true
					hop = h
					break
				}
			}
		}
	case ping.ResponseTimeout:
		return
	}
	if !ok {
		t.logger.Error("no hop stats for IP", "ip", resp.From)
		return
	}

	hop.recordRecv(int(resp.Request.Seq))
	if !hop.hasPinger {
		hop.hasPinger = true
		go t.startHopPinger(ctx, hop)
	}
}

// startHopPinger continuously pings a hop
func (t *Tracer) startHopPinger(ctx context.Context, hop *HopStats) {
	var seq int
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			seq++
			hop.recordSend(seq)
			t.logger.Debug("sending ping", "hop", hop.IP().String(), "seq", seq)
			_ = t.sock.Send(hop.IP(), ping.SequenceNumber(seq), 64, []byte("ping"))
		}
	}
}
