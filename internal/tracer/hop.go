package tracer

import (
	"net"
	"slices"
	"sync"
	"time"
)

// HopStats tracks stats per hop
type HopStats struct {
	sentTimes map[int]time.Time
	addr      string
	ip        net.IP
	RTTs      []time.Duration
	sent      int
	received  int
	mu        sync.Mutex
	TTL       uint8
	hasPinger bool
}

func (h *HopStats) IP() net.IP {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ip
}

func (h *HopStats) Addr() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.addr
}

// PacketCount returns the number of packets sent and received
func (h *HopStats) PacketCount() (int, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sent, h.received
}

// Loss returns the percentage (0-1) of packets lost
func (h *HopStats) Loss() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sent == 0 {
		return 0
	}
	return 1 - float64(h.received)/float64(h.sent)
}

// AvgRTT returns the average round trip time
func (h *HopStats) AvgRTT() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.RTTs) == 0 {
		return 0
	}
	var total time.Duration
	for _, r := range h.RTTs {
		total += r
	}
	return total / time.Duration(len(h.RTTs))
}

// MedianRTT returns the median round trip time
func (h *HopStats) MedianRTT() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := len(h.RTTs)
	if len(h.RTTs) == 0 {
		return 0
	}
	slices.Sort(h.RTTs)
	if n%2 == 1 {
		// Odd length, return the middle element
		return h.RTTs[n/2]
	}
	// Even length, return the average of the two middle elements
	return (h.RTTs[n/2-1] + h.RTTs[n/2]) / 2
}

func (h *HopStats) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sent = 0
	h.received = 0
	h.RTTs = h.RTTs[:0]
	clear(h.sentTimes)
}

func (h *HopStats) recordAddr(ip net.IP) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ip = ip
	var addr string
	if addresses, err := net.LookupAddr(h.ip.String()); err == nil && len(addresses) > 0 {
		addr = addresses[0]
	}
	h.addr = addr
}

func (h *HopStats) recordSend(seq int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sent++
	if h.sentTimes == nil {
		h.sentTimes = make(map[int]time.Time)
	}
	h.sentTimes[seq] = time.Now()
}

func (h *HopStats) recordRecv(seq int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.received++
	if t, ok := h.sentTimes[seq]; ok {
		h.RTTs = append(h.RTTs, time.Since(t))
		delete(h.sentTimes, seq)
	}
}
