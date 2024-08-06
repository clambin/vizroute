package icmp

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"testing"
	"time"
)

func TestPath_Discover_And_Ping_IPv4(t *testing.T) {
	l := slog.Default()
	s := New(IPv4, l)
	ctx, cancel := context.WithCancel(context.Background())

	addr, err := s.Resolve("localhost")
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", addr.String())

	var path Path
	require.NoError(t, path.Discover(ctx, s, addr))
	assert.Len(t, path.Hops(), 1)

	ch := make(chan struct{})
	go func() { path.Ping(ctx, s, l); ch <- struct{}{} }()

	assert.Eventually(t, func() bool {
		hops := path.Hops()
		if len(hops) != 1 {
			return false
		}
		return hops[0].Availability() == 1
	}, time.Second, time.Millisecond)

	cancel()
	<-ch

	assert.Equal(t, 1.0, path.Hops()[0].Availability())
	assert.NotZero(t, path.Hops()[0].Latency())
}

func TestPath_Discover_And_Ping_IPv6(t *testing.T) {
	t.Skip("broken")

	l := slog.Default()
	s := New(IPv6, l)
	ctx, cancel := context.WithCancel(context.Background())

	addr, err := s.Resolve("localhost")
	require.NoError(t, err)
	assert.Equal(t, "::1", addr.String())

	var path Path
	require.NoError(t, path.Discover(ctx, s, addr))
	assert.Len(t, path.Hops(), 1)

	ch := make(chan struct{})
	go func() { path.Ping(ctx, s, l); ch <- struct{}{} }()

	assert.Eventually(t, func() bool {
		hops := path.Hops()
		if len(hops) != 1 {
			return false
		}
		return hops[0].Availability() == 1
	}, time.Second, time.Millisecond)

	cancel()
	<-ch

	assert.Equal(t, 1.0, path.Hops()[0].Availability())
	assert.NotZero(t, path.Hops()[0].Latency())
}
