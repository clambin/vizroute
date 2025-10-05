package tracer

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/clambin/vizroute/ping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracer(t *testing.T) {
	s := fakeSocket{
		hops: map[int]net.IP{
			1: net.ParseIP("192.168.0.1"),
			2: net.ParseIP("192.168.1.1"),
			4: net.ParseIP("192.168.2.1"),
		},
		hosts: map[string]net.IP{
			"target": net.ParseIP("192.168.2.1"),
		},
	}
	target := "target"
	maxHops := 4
	tracer := NewTracer(&s, slog.New(slog.DiscardHandler))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err := tracer.Run(ctx, target, maxHops)
		require.NoError(t, err)
	}()

	var hops []*HopStats
	require.Eventually(t, func() bool {
		hops = tracer.Hops()
		if len(hops) != 4 {
			return false
		}
		_, rcvd := hops[3].PacketCount()
		return rcvd > 0
	}, 5*time.Second, 10*time.Millisecond)

	want := []struct {
		ttl  uint8
		ip   string
		addr string
	}{
		{1, "192.168.0.1", ""},
		{2, "192.168.1.1", ""},
		{3, "<nil>", ""},
		{4, "192.168.2.1", ""},
	}

	for i := range want {
		assert.Equal(t, want[i].ttl, hops[i].TTL)
		assert.Equal(t, want[i].ip, hops[i].IP().String())
		assert.Equal(t, want[i].addr, hops[i].Addr())
		sent, _ := hops[i].PacketCount()
		assert.NotZero(t, sent)
	}

}

var _ Socket = (*fakeSocket)(nil)

type fakeSocket struct {
	qeueue []ping.Response
	lock   sync.Mutex
	hosts  map[string]net.IP
	hops   map[int]net.IP
}

func (f *fakeSocket) Resolve(host string) (net.IP, error) {
	if addr, ok := f.hosts[host]; ok {
		return addr, nil
	}
	return nil, fmt.Errorf("host not found")
}

func (f *fakeSocket) Read(ctx context.Context) (ping.Response, error) {
	for {
		if r, err := f.pop(); err == nil {
			return r, nil
		}
		select {
		case <-ctx.Done():
			return ping.Response{}, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (f *fakeSocket) Send(ip net.IP, seq ping.SequenceNumber, ttl uint8, _ []byte) error {
	// is the target reachable for this ttl value?
	for i, hop := range f.hops {
		if hop.Equal(ip) && i <= int(ttl) {
			f.push(ping.Response{
				ResponseType: ping.ResponseEchoReply,
				Latency:      time.Millisecond * 100,
				From:         hop,
				Request: ping.Request{
					TTL:      ttl,
					Seq:      seq,
					TimeSent: time.Now(),
				},
			})
			return nil
		}
	}

	// no reachable host found. return time exceeded for the hop at ttl
	if hop, ok := f.hops[int(ttl)]; ok {
		f.push(ping.Response{
			ResponseType: ping.ResponseTimeExceeded,
			Latency:      time.Millisecond * 100,
			From:         hop,
			Request: ping.Request{
				TTL:      ttl,
				Seq:      seq,
				TimeSent: time.Now(),
			},
		})
	}
	return nil
}

func (f *fakeSocket) push(r ping.Response) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.qeueue = append(f.qeueue, r)
}

func (f *fakeSocket) pop() (ping.Response, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if len(f.qeueue) > 0 {
		r := f.qeueue[0]
		f.qeueue = f.qeueue[1:]
		return r, nil
	}
	return ping.Response{}, fmt.Errorf("queue is empty")
}
