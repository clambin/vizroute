package ping

import (
	"context"
	"fmt"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
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
	msgType icmp.Type
	seq     uint16
}

func (p *Path) Ping(ctx context.Context, s Socket, l *slog.Logger) error {
	pingHops(ctx, p.hops, s, time.Second, 5*time.Second, l)
	return nil
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
	addr      net.IP
	sent      float64
	ups       float64
	latencies time.Duration
	lock      sync.RWMutex
}

func (h *Hop) Sent() {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.sent++
}

func (h *Hop) Received(up bool, latency time.Duration) {
	if up {
		h.lock.Lock()
		defer h.lock.Unlock()
		h.ups++
		h.latencies += latency
	}
}

func (h *Hop) Addr() net.IP {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.addr
}

func (h *Hop) Packets() int {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return int(h.sent)
}

func (h *Hop) Availability() float64 {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if h.sent == 0 {
		return 0
	}
	return h.ups / h.sent
}

func (h *Hop) Latency() time.Duration {
	h.lock.RLock()
	defer h.lock.RUnlock()
	if h.ups == 0 {
		return 0
	}
	return h.latencies / time.Duration(h.ups)
}
