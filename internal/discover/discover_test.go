package discover

import (
	"context"
	"errors"
	icmp2 "github.com/clambin/pinger/pkg/ping/icmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"log/slog"
	"net"
	"os"
	"sync"
	"testing"
	"time"
)

func TestDiscover(t *testing.T) {
	l := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := fakeSocket{
		hops: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("127.0.0.2"), net.ParseIP("127.0.0.3")},
	}
	ip, err := net.ResolveIPAddr("ip", "127.0.0.3")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var route Path
	err = Discover(ctx, &route, ip.IP, &s, 20, l)
	require.NoError(t, err)
	assert.Equal(t, len(s.hops), route.Len())
}

var _ Socket = &fakeSocket{}

type fakeSocket struct {
	hops  []net.IP
	queue []icmp2.Response
	lock  sync.Mutex
}

func (f *fakeSocket) Ping(_ net.IP, seq icmp2.SequenceNumber, ttl uint8, payload []byte) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	idx := int(ttl) - 1
	msgType := ipv4.ICMPTypeTimeExceeded
	if idx >= len(f.hops)-1 {
		msgType = ipv4.ICMPTypeEchoReply
		idx = len(f.hops) - 1
	}
	f.queue = append(f.queue, icmp2.Response{
		From:     f.hops[idx],
		MsgType:  msgType,
		Body:     &icmp.Echo{Seq: int(seq), Data: payload},
		Received: time.Now(),
	})
	return nil
}

func (f *fakeSocket) Read(_ context.Context) (icmp2.Response, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if len(f.queue) == 0 {
		return icmp2.Response{}, errors.New("queue is empty")
	}
	response := f.queue[0]
	f.queue = f.queue[1:]
	return response, nil
}
