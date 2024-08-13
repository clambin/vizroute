package ping

import (
	"context"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
	"log/slog"
	"sync"
	"time"
)

func pingHop(ctx context.Context, hop *Hop, s Socket, interval, timeout time.Duration, ch chan response, l *slog.Logger) {
	sendTicker := time.NewTicker(interval)
	defer sendTicker.Stop()
	timeoutTicker := time.NewTicker(timeout)
	defer timeoutTicker.Stop()

	var packets outstandingPackets
	var seq uint16
	payload := make([]byte, 56)

	for {
		select {
		case <-sendTicker.C:
			// send a ping
			seq++
			if err := s.Ping(hop.Addr(), seq, uint8(64), payload); err != nil {
				l.Warn("ping failed: %v", "err", err)
			}
			// record the outgoing packet
			packets.add(seq)
			hop.Sent()
			l.Debug("packet sent", "seq", seq)
		case <-timeoutTicker.C:
			// mark any old packets as timed out
			if timedOut := packets.timeout(timeout); len(timedOut) > 0 {
				for range timedOut {
					hop.Received(false, 0)
				}
				l.Debug("packets timed out", "packets", timedOut, "current", seq)
			}
		case resp := <-ch:
			l.Debug("packet received", "seq", resp.seq, "type", resp.msgType)
			// get latency for the received sequence nr. discard any old packets (we already count them during timeout)
			if latency, ok := packets.latency(resp.seq); ok {
				// is the host up?
				up := ok && (resp.msgType == ipv4.ICMPTypeEchoReply || resp.msgType == ipv6.ICMPTypeEchoReply)
				// measure the state & latency
				hop.Received(up, latency)
				l.Debug("hop measured", "up", up, "latency", latency, "ok", ok)
			}
		case <-ctx.Done():
			return
		}
	}
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////

type outstandingPackets struct {
	packets map[uint16]time.Time
	lock    sync.Mutex
}

func (o *outstandingPackets) add(seq uint16) {
	o.lock.Lock()
	defer o.lock.Unlock()
	if o.packets == nil {
		o.packets = make(map[uint16]time.Time)
	}
	o.packets[seq] = time.Now()
}

func (o *outstandingPackets) latency(seq uint16) (time.Duration, bool) {
	o.lock.Lock()
	defer o.lock.Unlock()
	sent, ok := o.packets[seq]
	if ok {
		delete(o.packets, seq)
	}
	return time.Since(sent), ok
}

func (o *outstandingPackets) timeout(timeout time.Duration) []uint16 {
	o.lock.Lock()
	defer o.lock.Unlock()
	var timedOut []uint16
	for seq, sent := range o.packets {
		if time.Since(sent) > timeout {
			delete(o.packets, seq)
			timedOut = append(timedOut, seq)
		}
	}
	return timedOut
}

///////////////////////////////////////////////////////////////////////////////////////////////////////////

func pingHops(ctx context.Context, hops []*Hop, s Socket, interval, timeout time.Duration, l *slog.Logger) {
	responses := make(map[string]chan response)
	for _, hop := range hops {
		if hop != nil && hop.Addr().String() != "" {
			responses[hop.Addr().String()] = make(chan response, 1)
		}
	}
	go receiveResponses(ctx, s, responses, l)
	for _, hop := range hops {
		if hop != nil {
			if ch, ok := responses[hop.Addr().String()]; ok {
				go pingHop(ctx, hop, s, interval, timeout, ch, l.With("addr", hop.Addr()))
			}
		}
	}
	<-ctx.Done()
}

func receiveResponses(ctx context.Context, s Socket, responses map[string]chan response, l *slog.Logger) {
	for {
		addr, msgType, seq, err := s.Read(ctx)
		if err != nil {
			l.Warn("read failed", "err", err)
			continue
		}
		l.Debug("received packet", "addr", addr, "msgType", msgType, "seq", seq)
		ch, ok := responses[addr.String()]
		if !ok {
			l.Warn("no channel found for hop", "addr", addr, "msgType", msgType, "seq", seq)
			continue
		}
		ch <- response{
			msgType: msgType,
			seq:     seq,
		}

		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}
