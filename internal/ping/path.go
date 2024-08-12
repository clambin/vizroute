package ping

import (
	"context"
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"
)

type Path struct {
	MaxTTL int
	hops   []*Hop
	lock   sync.RWMutex
}

type Socket interface {
	Ping(net.IP, uint16, uint8, []byte) error
	Read(context.Context) (net.IP, icmp.Type, uint16, error)
}

func (p *Path) Run(ctx context.Context, s Socket, addr net.IP, l *slog.Logger) error {
	l.Debug("discovering hops")
	if err := p.Discover(ctx, s, addr, l); err != nil {
		return fmt.Errorf("discover: %w", err)
	}
	l.Debug("pinging hops", "count", len(p.Hops()))
	return p.Ping(ctx, s, l)
}

const defaultMaxTTL = 64

func (p *Path) Discover(ctx context.Context, s Socket, addr net.IP, l *slog.Logger) error {
	maxTTL := p.MaxTTL
	if maxTTL == 0 {
		maxTTL = defaultMaxTTL
	}

	var seq uint16
	payload := make([]byte, 56)
	for i := range maxTTL {
		ttl := uint8(1 + i)
		if err := s.Ping(addr, seq, ttl, payload); err != nil {
			return fmt.Errorf("ping: %w", err)
		}
		if from, msgType, _, err := s.Read(ctx); err == nil {
			l.Debug("hop discovered", "addr", from, "ttl", ttl)
			p.Add(ttl, from)
			if msgType == ipv4.ICMPTypeEchoReply || msgType == ipv6.ICMPTypeEchoReply {
				return nil
			}
		}
		seq++
	}
	return fmt.Errorf("no path found: max TTL (%d) exceeded", maxTTL+1)
}

type response struct {
	err     error
	msgType icmp.Type
	seq     uint16
}

func (p *Path) Ping(ctx context.Context, s Socket, l *slog.Logger) error {
	hops := make(map[string]chan response)
	for _, hop := range p.hops {
		if hop != nil && hop.Addr().String() != "" {
			hops[hop.Addr().String()] = make(chan response)
		}
	}

	var g errgroup.Group
	for _, hop := range p.hops {
		if hop != nil {
			if ch, ok := hops[hop.Addr().String()]; ok {
				g.Go(func() error {
					return p.pingHop(ctx, hop, s, ch, l)
				})
			}
		}
	}
	g.Go(func() error { return p.handleHopResponses(ctx, s, hops, l) })
	return g.Wait()
}

func (p *Path) pingHop(ctx context.Context, h *Hop, s Socket, ch chan response, l *slog.Logger) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var seq uint16
	var start time.Time
	payload := make([]byte, 56)
	for {
		select {
		case <-ticker.C:
			start = time.Now()
			seq++
			l.Debug("pinging hop", "addr", h.Addr(), "seq", seq)
			if err := s.Ping(h.Addr(), seq, 255, payload); err != nil {
				return fmt.Errorf("ping: %w", err)
			}
		case resp := <-ch:
			l.Debug("hop responded", "type", resp.msgType, "seq", resp.seq, "err", resp.err, "up", resp.err == nil && (resp.msgType == ipv4.ICMPTypeEchoReply || resp.msgType == ipv6.ICMPTypeEchoReply))
			if resp.err != nil || resp.seq == seq {
				h.Measure(resp.err == nil && (resp.msgType == ipv4.ICMPTypeEchoReply || resp.msgType == ipv6.ICMPTypeEchoReply), time.Since(start))
			}
			l.Debug("hop", "availability", h.Availability())
		case <-ctx.Done():
			return nil
		}
	}
}

func (p *Path) handleHopResponses(ctx context.Context, s Socket, hops map[string]chan response, l *slog.Logger) error {
	for {
		from, msgType, seq, err := s.Read(ctx)
		l.Debug("response received", "from", from.String(), "type", msgType, "seq", seq, "err", err)
		select {
		case <-ctx.Done():
			return nil
		default:
			if ch, ok := hops[from.String()]; ok {
				ch <- response{err: err, msgType: msgType, seq: seq}
			}
		}
	}
}

func (p *Path) MaxLatency() time.Duration {
	p.lock.RLock()
	defer p.lock.RUnlock()
	var maxLatency time.Duration
	for _, h := range p.hops {
		if h != nil {
			maxLatency = max(maxLatency, h.Latency())
		}
	}
	return maxLatency
}

func (p *Path) Add(hop uint8, from net.IP) *Hop {
	p.lock.Lock()
	defer p.lock.Unlock()
	for range int(hop) - len(p.hops) {
		p.hops = append(p.hops, nil)
	}
	h := Hop{addr: from}
	p.hops[hop-1] = &h
	return &h
}

func (p *Path) Hops() []*Hop {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return slices.Clone(p.hops)
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////

type Hop struct {
	addr         net.IP
	ups          float64
	upCount      float64
	latencies    time.Duration
	latencyCount float64
	lock         sync.RWMutex
}

func (h *Hop) Measure(up bool, latency time.Duration) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.upCount++
	if up {
		h.ups++
		h.latencies += latency
		h.latencyCount++
	}
}

func (h *Hop) Addr() net.IP {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.addr
}

func (h *Hop) Availability() float64 {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if h.upCount == 0 {
		return 0
	}
	return h.ups / h.upCount
}

func (h *Hop) Latency() time.Duration {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if h.latencyCount == 0 {
		return 0
	}
	return h.latencies / time.Duration(h.latencyCount)
}
