package request

import (
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"sort"
	"sync"
	"time"

	"github.com/runbrick/reqo/internal/model"
)

// sendTrace measures the entire redirect chain, but retains only the final
// hop's phases. Their Start offsets are relative to the entire send's start;
// time spent in earlier hops is consequently a gap in the final timeline.
type sendTrace struct {
	base  http.RoundTripper
	start time.Time
	last  *hopTrace
}

type interval struct {
	kind       model.PhaseKind
	start, end time.Time
}

// httptrace callbacks may run concurrently, including after RoundTrip returns.
type hopTrace struct {
	mu                            sync.Mutex
	dns, connect, tls, send, wait interval
	connects                      map[string]time.Time
	firstByte                     time.Time
}

func (s *sendTrace) RoundTrip(req *http.Request) (*http.Response, error) {
	h := &hopTrace{connects: make(map[string]time.Time)}
	s.last = h
	trace := &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.dns = interval{kind: model.PhaseDNS, start: time.Now()}
		},
		DNSDone: func(httptrace.DNSDoneInfo) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.dns.end = time.Now()
		},
		ConnectStart: func(network, addr string) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.connects[network+"\x00"+addr] = time.Now()
		},
		ConnectDone: func(network, addr string, err error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			if err == nil && h.connect.end.IsZero() {
				h.connect = interval{model.PhaseConnect, h.connects[network+"\x00"+addr], time.Now()}
			}
		},
		TLSHandshakeStart: func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.tls = interval{kind: model.PhaseTLS, start: time.Now()}
		},
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.tls.end = time.Now()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			h.mu.Lock()
			defer h.mu.Unlock()
			if info.Reused {
				h.dns, h.connect, h.tls = interval{}, interval{}, interval{}
			}
			h.send = interval{kind: model.PhaseSend, start: time.Now()}
		},
		WroteRequest: func(httptrace.WroteRequestInfo) {
			h.mu.Lock()
			defer h.mu.Unlock()
			now := time.Now()
			h.send.end = now
			h.wait = interval{model.PhaseWait, now, h.firstByte}
		},
		GotFirstResponseByte: func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.firstByte = time.Now()
			h.wait.end = h.firstByte
		},
	}
	return s.base.RoundTrip(req.WithContext(httptrace.WithClientTrace(req.Context(), trace)))
}

func (s *sendTrace) timeline(downloadStart, end time.Time) []model.Phase {
	h := s.last
	h.mu.Lock()
	intervals := []interval{h.dns, h.connect, h.tls, h.send, h.wait,
		{model.PhaseDownload, downloadStart, end}}
	h.mu.Unlock()
	sort.SliceStable(intervals, func(i, j int) bool { return intervals[i].start.Before(intervals[j].start) })
	var phases []model.Phase
	previousEnd := time.Duration(0)
	duration := end.Sub(s.start)
	for _, p := range intervals {
		if p.start.IsZero() || p.end.IsZero() {
			continue
		}
		// DNS/dials can overlap, and an early response can arrive before the
		// upload finishes. Attribute overlapping time once, in start order.
		phaseStart := min(duration, max(previousEnd, p.start.Sub(s.start)))
		phaseEnd := min(duration, max(phaseStart, p.end.Sub(s.start)))
		phases = append(phases, model.Phase{Kind: p.kind, Start: phaseStart, Duration: phaseEnd - phaseStart})
		previousEnd = phaseEnd
	}
	return phases
}
