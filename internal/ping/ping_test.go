package ping

import (
	"context"
	"github.com/clambin/vizroute/internal/ping/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"log/slog"
	"net"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func Test_pingHop(t *testing.T) {
	addr := net.ParseIP("127.0.0.1")
	h := Hop{addr: addr}
	ch := make(chan response, 1)
	s := mocks.NewSocket(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const latency = 100 * time.Millisecond
	s.EXPECT().
		Ping(addr, mock.Anything, uint8(0x40), mock.Anything).
		RunAndReturn(func(_ net.IP, seq uint16, _ uint8, _ []byte) error {
			time.Sleep(latency)
			ch <- response{seq: seq, msgType: ipv4.ICMPTypeEchoReply}
			return nil
		})
	l := slog.Default() // slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	go pingHop(ctx, &h, s, time.Second, 5*time.Second, ch, l)

	assert.Eventually(t, func() bool {
		return h.Availability() > 0
	}, 5*time.Second, 10*time.Millisecond)
	assert.LessOrEqual(t, h.Latency(), latency)
}

func Test_pingHops(t *testing.T) {
	addr := net.ParseIP("127.0.0.1")
	h := Hop{addr: addr}
	s := mocks.NewSocket(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var lastSeq atomic.Int32
	s.EXPECT().
		Ping(addr, mock.Anything, uint8(0x40), mock.Anything).
		RunAndReturn(func(_ net.IP, seq uint16, _ uint8, _ []byte) error {
			lastSeq.Store(int32(seq))
			return nil
		})
	const latency = 500 * time.Millisecond
	s.EXPECT().Read(ctx).RunAndReturn(func(ctx context.Context) (net.IP, icmp.Type, uint16, error) {
		time.Sleep(latency)
		return addr, ipv4.ICMPTypeEchoReply, uint16(lastSeq.Load()), nil
	})

	l := slog.Default() // slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	go pingHops(ctx, []*Hop{&h}, s, time.Second, 5*time.Second, l)

	assert.Eventually(t, func() bool {
		return h.Availability() > 0
	}, 5*time.Second, 10*time.Millisecond)
	assert.LessOrEqual(t, h.Latency(), 2*latency)
}

func Test_outstandingPackets_timeout(t *testing.T) {
	var p outstandingPackets
	p.add(1)
	p.add(2)
	time.Sleep(time.Second)
	p.add(3)
	timedOut := p.timeout(500 * time.Millisecond)
	slices.Sort(timedOut)
	assert.Equal(t, []uint16{1, 2}, timedOut)
	assert.Len(t, p.packets, 1)
	_, ok := p.packets[3]
	assert.True(t, ok)
}
