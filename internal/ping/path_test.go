package ping

import (
	"context"
	"github.com/clambin/vizroute/internal/icmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
	"time"
)

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
