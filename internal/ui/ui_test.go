package ui

import (
	"context"
	"github.com/clambin/vizroute/internal/ping"
	"github.com/clambin/vizroute/internal/ui/mocks"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestUI_Update(t *testing.T) {
	a := mocks.NewApplication(t)
	var called atomic.Bool
	a.EXPECT().QueueUpdateDraw(mock.AnythingOfType("func()")).RunAndReturn(func(f func()) *tview.Application {
		f()
		called.Store(true)
		return nil
	})

	var path ping.Path
	path.Add(1, net.ParseIP("1.1.1.1"))
	path.Hops()[0].Sent()
	path.Hops()[0].Received(true, time.Second)
	tui := New(&path, true)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		tui.Update(ctx, a, 10*time.Millisecond)
		done <- struct{}{}
	}()

	assert.Eventually(t, func() bool { return called.Load() }, time.Second, 10*time.Millisecond)
	cancel()
	<-done

	content := readTable(tui.table)
	assert.Equal(t, [][]string{
		{"hop", "addr", "name", "sent", "rcvd", "latency", "", "loss", ""},
		{"1", "1.1.1.1", "one.one.one.one.", "1", "1", "1000.0ms", "|**********|", "0.0%", "|----------|"},
	}, content)
}
