package icmp

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestSocket_Ping_IPv4(t *testing.T) {
	s := New(IPv4, slog.Default())
	ip, err := s.Resolve("127.0.0.1")
	require.NoError(t, err)
	require.NoError(t, s.Ping(ip, 1, 255, []byte("payload")))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, _, _, err = s.Read(ctx)
	assert.NoError(t, err)
}

func TestSocket_Ping_IPv6(t *testing.T) {
	//t.Skip("IPv6 broken")
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := New(IPv6, l)
	ip, err := s.Resolve("::1")
	require.NoError(t, err)
	require.NoError(t, s.Ping(ip, 1, 0, []byte("payload")))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, _, _, err = s.Read(ctx)
	assert.NoError(t, err)
}

func TestTransport_String(t *testing.T) {
	tests := []struct {
		name string
		tp   Transport
		want string
	}{
		{name: "IPv4", tp: IPv4, want: "ipv4"},
		{name: "IPv6", tp: IPv6, want: "ipv6"},
		{name: "unknown", tp: -1, want: "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.tp.String())
		})
	}
}

func TestSocket_Resolve(t *testing.T) {
	tests := []struct {
		name    string
		tp      Transport
		addr    string
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "IPv4",
			tp:      IPv4,
			addr:    "localhost",
			want:    "127.0.0.1",
			wantErr: assert.NoError,
		},
		{
			name:    "IPv6",
			tp:      IPv6,
			addr:    "localhost",
			want:    "::1",
			wantErr: assert.NoError,
		},
		{
			name:    "IPv6 not supported",
			tp:      IPv4,
			addr:    "::1",
			want:    "<nil>",
			wantErr: assert.Error,
		},
		{
			name:    "invalid hostname",
			tp:      IPv4,
			addr:    "",
			want:    "<nil>",
			wantErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := New(tt.tp, slog.Default())
			addr, err := s.Resolve(tt.addr)
			assert.Equal(t, tt.want, addr.String())
			tt.wantErr(t, err)
		})
	}
}
