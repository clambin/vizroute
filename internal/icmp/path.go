package icmp

import (
	"context"
	"fmt"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"log/slog"
	"net"
	"slices"
	"sync"
	"time"
)

type Path struct {
	hops []*Hop
	lock sync.RWMutex
}

func (p *Path) Discover(ctx context.Context, s *Socket, addr net.IP) error {
	var seq uint16
	var ttl uint8
	for {
		ttl++
		if err := s.Ping(addr, seq, ttl, []byte("payload")); err != nil {
			return fmt.Errorf("ping: %w", err)
		}
		from, msgType, err := s.Read(ctx)
		if err == nil {
			p.Add(int(ttl), from)
		}
		seq++
		if msgType == ipv4.ICMPTypeEchoReply || msgType == ipv6.ICMPTypeEchoReply {
			break
		}
	}
	return nil
}

func (p *Path) Ping(ctx context.Context, s *Socket, l *slog.Logger) {
	var i int
	var seq uint16

	ticker := time.NewTicker(100 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if hops := p.Hops(); i < len(hops) {
				if h := hops[i]; h != nil {
					if err := p.pingHop(ctx, h, s, seq); err != nil {
						l.Warn("ping failed", "err", err)
					}
				}
				i = (i + 1) % len(hops)
			}
		}
	}
}

func (p *Path) pingHop(ctx context.Context, h *Hop, s *Socket, seq uint16) error {
	if h == nil {
		return nil
	}
	if addr := h.Addr(); addr != nil {
		start := time.Now()
		if err := s.Ping(addr, seq, 255, []byte("payload")); err != nil {
			return fmt.Errorf("pingHop: %w", err)
		}
		_, msgType, err := s.Read(ctx)
		h.Measurement(err == nil && (msgType == ipv4.ICMPTypeEchoReply || msgType == ipv6.ICMPTypeEchoReply), time.Since(start))
	}
	return nil
}

func (p *Path) MaxLatency() time.Duration {
	p.lock.RLock()
	defer p.lock.RUnlock()
	var maxLatency time.Duration
	for _, h := range p.hops {
		if h != nil {
			if latency := h.Latency(); latency > maxLatency {
				maxLatency = latency
			}
		}
	}
	return maxLatency
}

func (p *Path) Add(hop int, from net.IP) *Hop {
	p.lock.Lock()
	defer p.lock.Unlock()
	for range hop - len(p.hops) {
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

func (h *Hop) Measurement(up bool, latency time.Duration) {
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
