package discover

import (
	"context"
	"fmt"
	"github.com/clambin/pinger/pkg/ping"
	"github.com/clambin/pinger/pkg/ping/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"log/slog"
	"net"
	"sync"
)

type Path struct {
	Hops []*ping.Target
	lock sync.RWMutex
}

func (p *Path) AddHop() {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.Hops = append(p.Hops, nil)
}

func (p *Path) SetHop(idx int, hop *ping.Target) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.Hops[idx] = hop
}

func (p *Path) Len() int {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return len(p.Hops)
}

type Socket interface {
	Ping(net.IP, icmp.SequenceNumber, uint8, []byte) error
	Read(context.Context) (icmp.Response, error)
}

func Discover(ctx context.Context, route *Path, addr net.IP, s Socket, maxTTL uint8, l *slog.Logger) error {
	const defaultMaxTTL = 64
	if maxTTL == 0 {
		maxTTL = defaultMaxTTL
	}

	var seq icmp.SequenceNumber
	payload := make([]byte, 56)
	for range maxTTL {
		route.AddHop()
		ttl := uint8(route.Len())
		if err := s.Ping(addr, seq, ttl, payload); err != nil {
			return fmt.Errorf("ping: %w", err)
		}
		if resp, err := s.Read(ctx); err == nil {
			l.Debug("hop discovered", "addr", resp.From, "ttl", ttl)
			route.SetHop(int(ttl-1), &ping.Target{IP: resp.From})
			if resp.MsgType == ipv4.ICMPTypeEchoReply || resp.MsgType == ipv6.ICMPTypeEchoReply {
				return nil
			}
		}
		seq++
	}
	return fmt.Errorf("no path found: max TTL (%d) exceeded", maxTTL+1)
}
