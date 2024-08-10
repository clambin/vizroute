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
	//l := slog.Default()
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := icmp.New(icmp.IPv4, l)
	ctx, cancel := context.WithCancel(context.Background())

	addr, err := s.Resolve("localhost")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", addr.String())

	var path Path
	require.NoError(t, path.Discover(ctx, s, addr))
	assert.Len(t, path.Hops(), 1)

	ch := make(chan struct{})
	go func() {
		if err := path.Ping(ctx, s, l); err != nil {
			panic(err)
		}
		ch <- struct{}{}
	}()

	require.Eventually(t, func() bool {
		hops := path.Hops()
		if len(hops) != 1 {
			return false
		}
		return hops[0].Availability() == 1
	}, 5*time.Second, time.Millisecond)

	cancel()
	<-ch

	assert.Equal(t, 1.0, path.Hops()[0].Availability())
	assert.NotZero(t, path.Hops()[0].Latency())
}

func TestPath_Discover_And_Ping_IPv6(t *testing.T) {
	//t.Skip("broken")

	//l := slog.Default()
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := icmp.New(icmp.IPv6, l)
	ctx, cancel := context.WithCancel(context.Background())

	addr, err := s.Resolve("localhost")
	require.NoError(t, err)
	assert.Equal(t, "::1", addr.String())

	var path Path
	require.NoError(t, path.Discover(ctx, s, addr))
	assert.Len(t, path.Hops(), 1)

	ch := make(chan struct{})
	go func() {
		if err := path.Ping(ctx, s, l); err != nil {
			panic(err)
		}
		ch <- struct{}{}
	}()

	require.Eventually(t, func() bool {
		hops := path.Hops()
		if len(hops) != 1 {
			return false
		}
		return hops[0].Availability() == 1
	}, 5*time.Second, time.Millisecond)

	cancel()
	<-ch

	assert.Equal(t, 1.0, path.Hops()[0].Availability())
	assert.NotZero(t, path.Hops()[0].Latency())
}
