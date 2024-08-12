package ping

import (
	"context"
	"errors"
	"github.com/clambin/vizroute/internal/icmp"
	"github.com/clambin/vizroute/internal/ping/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	icmp2 "golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func TestPath_Ping(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx, cancel := context.WithCancel(context.Background())
	s := mocks.NewSocket(t)
	var lastSeq atomic.Uint32
	var shouldReply atomic.Int32
	localhost := net.ParseIP("127.0.0.1")

	s.EXPECT().Ping(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ net.IP, seq uint16, _ uint8, _ []byte) error {
		lastSeq.Store(uint32(seq))
		shouldReply.Add(1)
		return nil
	})
	s.EXPECT().Read(ctx).RunAndReturn(func(_ context.Context) (net.IP, icmp2.Type, uint16, error) {
		if shouldReply.Load() > 0 {
			if seq := lastSeq.Load(); seq%2 != 0 {
				time.Sleep(10 * time.Millisecond)
				shouldReply.Add(-1)
				return localhost, ipv4.ICMPTypeEchoReply, uint16(seq), nil
			}
		}
		time.Sleep(time.Second)
		if shouldReply.Load() > 0 {
			return localhost, ipv4.ICMPTypeTimeExceeded, 0, errors.New("no answer")
		}
		return nil, ipv4.ICMPTypeEchoReply, 0, errors.New("timeout")
	})

	path := Path{
		hops: []*Hop{{addr: localhost}},
	}

	ch := make(chan error)
	go func() { ch <- path.Ping(ctx, s, l) }()

	assert.Eventually(t, func() bool {
		hops := path.Hops()
		return hops[0].Availability() == 0.5
	}, 5*time.Second, 100*time.Millisecond)

	cancel()
	assert.NoError(t, <-ch)
}

func TestPath_Discover_And_Ping_IPv4(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := icmp.New(icmp.IPv4, l.With("socket", "ipv4"))
	ctx, cancel := context.WithCancel(context.Background())

	addr, err := s.Resolve("localhost")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", addr.String())

	var path Path
	ch := make(chan error)
	go func() {
		ch <- path.Run(ctx, s, addr, l)
	}()

	require.Eventually(t, func() bool {
		if hops := path.Hops(); len(hops) == 1 {
			return hops[0].Availability() == 1
		}
		return false
	}, 5*time.Second, time.Millisecond)

	cancel()
	assert.NoError(t, <-ch)
	assert.Equal(t, 1.0, path.Hops()[0].Availability())
	assert.NotZero(t, path.Hops()[0].Latency())
}

func TestPath_Discover_And_Ping_IPv6(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := icmp.New(icmp.IPv6, l.With("socket", "ipv6"))
	ctx, cancel := context.WithCancel(context.Background())

	addr, err := s.Resolve("localhost")
	require.NoError(t, err)
	assert.Equal(t, "::1", addr.String())

	var path Path
	ch := make(chan error)
	go func() {
		ch <- path.Run(ctx, s, addr, l)
	}()

	require.Eventually(t, func() bool {
		if hops := path.Hops(); len(hops) == 1 {
			return hops[0].Availability() == 1
		}
		return false
	}, 5*time.Second, time.Millisecond)

	cancel()
	assert.NoError(t, <-ch)
	assert.Equal(t, 1.0, path.Hops()[0].Availability())
	assert.NotZero(t, path.Hops()[0].Latency())
}

func TestPath_MaxLatency(t *testing.T) {
	type hop struct {
		hop     int
		latency time.Duration
	}
	tests := []struct {
		name string
		hops []hop
		want time.Duration
	}{
		{
			name: "empty",
			hops: nil,
			want: 0,
		},
		{
			name: "not empty",
			hops: []hop{
				{hop: 1, latency: 1000 * time.Millisecond},
				{hop: 2, latency: 1500 * time.Millisecond},
				{hop: 3, latency: 1200 * time.Millisecond},
			},
			want: 1500 * time.Millisecond,
		},
		{
			name: "missing hop",
			hops: []hop{
				{hop: 1, latency: 1000 * time.Millisecond},
				{hop: 2, latency: 0},
				{hop: 3, latency: 1200 * time.Millisecond},
			},
			want: 1200 * time.Millisecond,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var p Path
			for _, h := range tt.hops {
				hop := p.Add(uint8(h.hop), nil)
				if h.latency > 0 {
					hop.Measure(true, h.latency)
				}
			}
			assert.Equal(t, tt.want, p.MaxLatency())
		})
	}
}

func TestHop_Measure(t *testing.T) {
	var h Hop

	h.Measure(false, 0)
	assert.Equal(t, 1, h.Packets())
	assert.Equal(t, float64(0), h.Availability())
	assert.Equal(t, time.Duration(0), h.Latency())

	h.Measure(true, time.Second)
	assert.Equal(t, 2, h.Packets())
	assert.Equal(t, 0.5, h.Availability())
	assert.Equal(t, time.Second, h.Latency())

	h.Measure(true, 3*time.Second)
	assert.Equal(t, 3, h.Packets())
	assert.Equal(t, 2.0/3.0, h.Availability())
	assert.Equal(t, 2*time.Second, h.Latency())
}
