package ping

import (
	"encoding/binary"
	"errors"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func TestSocket(t *testing.T) {
	tests := []struct {
		name    string
		network string
		address string
		target  string
		tp      Transport
	}{
		{"IPv4", "udp4", "8.8.8.8:53", "127.0.0.1", IPv4},
		{"IPv6", "udp6", "[2001:4860:4860::8888]:53", "::1", IPv6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// check if we have the required IP version
			if _, err := net.Dial(tt.network, tt.address); err != nil {
				t.Skip("IP version not supported")
			}

			socket, err := New(tt.tp, slog.New(slog.DiscardHandler))
			if errors.Is(err, os.ErrPermission) {
				t.Skip("permission denied")
			}
			require.NoError(t, err)

			ctx := t.Context()
			go socket.Serve(ctx)

			target, err := socket.Resolve(tt.target)
			require.NoError(t, err)

			err = socket.Send(target, 1, 255, []byte("payload"))
			require.NoError(t, err)

			resp, err := socket.Read(ctx)
			require.NoError(t, err)
			assert.Equal(t, ResponseEchoReply, resp.ResponseType)
			assert.Equal(t, tt.target, resp.From.String())
			assert.Equal(t, SequenceNumber(1), resp.Request.Seq)
		})
	}
}

func TestResponse_LogValue(t *testing.T) {
	tests := []struct {
		name string
		resp Response
		want string
	}{
		{
			name: "timeout",
			resp: Response{ResponseType: ResponseTimeout},
			want: `[type=timeout]`,
		},
		{
			name: "time exceeded",
			resp: Response{ResponseType: ResponseTimeExceeded, From: net.ParseIP("192.168.0.1"), Request: Request{Target: net.ParseIP("1.1.1.1"), Seq: 10, TTL: 1}},
			want: `[type=time exceeded from=192.168.0.1 target=1.1.1.1 seq=10 ttl=1]`,
		},
		{
			name: "echo reply",
			resp: Response{ResponseType: ResponseEchoReply, From: net.ParseIP("192.168.0.1"), Request: Request{Target: net.ParseIP("192.168.0.1"), Seq: 2, TTL: 64}},
			want: `[type=echo reply from=192.168.0.1 target=192.168.0.1 seq=2 ttl=64]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.resp.LogValue().String())
		})
	}
}

func TestParseTimeExceededV4_Success(t *testing.T) {
	const (
		id  = 0x1234
		seq = 0x5678
	)

	// Fake IPv4 header (20 bytes, IHL=5)
	ipHeader := make([]byte, ipv4.HeaderLen)
	ipHeader[0] = (4 << 4) | 5 // Version 4, IHL=5 (20 bytes)

	// Inner ICMP Echo header (8 bytes)
	inner := make([]byte, 8)
	binary.BigEndian.PutUint16(inner[4:], uint16(id))
	binary.BigEndian.PutUint16(inner[6:], uint16(seq))

	packet := append(ipHeader, inner...)

	gotID, gotSeq, err := parseTimeExceededV4(packet)
	require.NoError(t, err)
	assert.Equal(t, id, gotID)
	assert.Equal(t, SequenceNumber(seq), gotSeq)
}

func TestParseTimeExceededV4_ShortPayload(t *testing.T) {
	_, _, err := parseTimeExceededV4([]byte{0x45})
	require.Error(t, err)
	require.Contains(t, err.Error(), "too short")
}

func TestParseTimeExceededV6_Success_Echo(t *testing.T) {
	const (
		id  = 0xabcd
		seq = 0x42
	)

	// Build ICMPv6 Echo request
	echo := &icmp.Echo{
		ID:  id,
		Seq: seq,
	}
	msg := &icmp.Message{
		Type: ipv6.ICMPTypeEchoRequest,
		Code: 0,
		Body: echo,
	}
	raw, err := msg.Marshal(nil)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Prepend IPv6 header
	data := append(make([]byte, ipv6.HeaderLen), raw...)

	gotID, gotSeq, err := parseTimeExceededV6(data)
	require.NoError(t, err)
	assert.Equal(t, id, gotID)
	assert.Equal(t, SequenceNumber(seq), gotSeq)
}

func TestParseTimeExceededV6_ShortPayload(t *testing.T) {
	_, _, err := parseTimeExceededV6(make([]byte, ipv6.HeaderLen-1))
	require.Error(t, err)
	require.Contains(t, err.Error(), "too short")
}

func TestParseTimeExceededV6_FallbackToRawBytes(t *testing.T) {
	const (
		id  = 0x1234
		seq = 0xabcd
	)

	// IPv6 header
	hdr := make([]byte, ipv6.HeaderLen)
	// inner payload that isn't a valid ICMP message, but is long enough
	inner := make([]byte, 8)
	binary.BigEndian.PutUint16(inner[4:], uint16(id))
	binary.BigEndian.PutUint16(inner[6:], uint16(seq))
	data := append(hdr, inner...)

	gotID, gotSeq, err := parseTimeExceededV6(data)
	require.NoError(t, err)
	assert.Equal(t, id, gotID)
	assert.Equal(t, SequenceNumber(seq), gotSeq)
}
