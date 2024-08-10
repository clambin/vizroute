package icmp

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
		from, msgType, _, err := s.Read(ctx)
		if err == nil {
			p.Add(int(ttl), from)
		}
		if msgType == ipv4.ICMPTypeEchoReply || msgType == ipv6.ICMPTypeEchoReply {
			break
		}
		seq++
	}
	return nil
}

type icmpResponse struct {
	icmp.Type
	Seq uint16
}

func (p *Path) Ping(ctx context.Context, s *Socket, l *slog.Logger) error {
	pingers := make(map[string]chan icmpResponse)
	for _, hop := range p.hops {
		if hop != nil && hop.Addr().String() != "" {
			pingers[hop.Addr().String()] = make(chan icmpResponse)
		}
	}

	var g errgroup.Group
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				if from, msgType, seq, err := s.Read(ctx); err == nil {
					l.Debug("packet received", "from", from.String(), "seq", seq)
					if ch, ok := pingers[from.String()]; ok {
						ch <- icmpResponse{Type: msgType, Seq: seq}
					}
				}
			}
		}
	})
	for _, hop := range p.hops {
		if hop != nil {
			if ch, ok := pingers[hop.Addr().String()]; ok {
				g.Go(func() error { return p.pingHop(ctx, hop, s, ch) })
			}
		}
	}
	return g.Wait()
}

func (p *Path) pingHop(ctx context.Context, h *Hop, s *Socket, ch chan icmpResponse) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var seq uint16
	var start time.Time
	for {
		select {
		case <-ticker.C:
			start = time.Now()
			seq++
			if err := s.Ping(h.Addr(), seq, 255, []byte("payload")); err != nil {
				return fmt.Errorf("ping: %w", err)
			}
		case resp := <-ch:
			if resp.Seq == seq {
				h.Measurement(resp.Type == ipv4.ICMPTypeEchoReply || resp.Type == ipv6.ICMPTypeEchoReply, time.Since(start))
			}
		case <-ctx.Done():
			return nil
		}
	}
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
