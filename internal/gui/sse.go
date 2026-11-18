package gui

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/request"
	"github.com/vivaui/viva/ui"
)

const (
	maxSSEEvents = 64
	maxSSEBytes  = 128 << 10
)

type sseSender interface {
	SendSSE(context.Context, model.Request, []model.KV, request.SSEObserver) (model.Response, error)
}

type ssePlanSender struct {
	sender  sseSender
	stream  *sseMailbox
	partial *model.Response
}

func (s *ssePlanSender) Send(ctx context.Context, req model.Request, vars []model.KV) (model.Response, error) {
	opened := false
	response, err := s.sender.SendSSE(ctx, req, vars, request.SSEObserver{
		Opened: func(statusCode int) {
			opened = true
			s.stream.open(statusCode)
		},
		Event: s.stream.push,
	})
	if opened && err != nil {
		s.partial = &response
	}
	return response, err
}

type sseBatch struct {
	status   int
	events   []request.SSEEvent
	overflow bool
}

// sseMailbox has one pending wake and a lifetime budget shared across drains.
// Only the worker publishes. The UI drains under the same short lock so a
// stale wake cannot consume a later publication.
type sseMailbox struct {
	mu           sync.Mutex
	ready        chan struct{}
	pending      sseBatch
	count, bytes int
	overflow     bool
	closed       bool
}

func (m *sseMailbox) notify() {
	select {
	case m.ready <- struct{}{}:
	default:
	}
}

func (m *sseMailbox) open(statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.pending.status = statusCode
		m.notify()
	}
}

func (m *sseMailbox) push(event request.SSEEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.overflow {
		return
	}
	size := len(event.Type) + len(event.ID) + len(event.Data)
	if m.count == maxSSEEvents || size > maxSSEBytes-m.bytes {
		m.overflow, m.pending.overflow = true, true
	} else {
		m.pending.events = append(m.pending.events, event)
		m.bytes += size
		m.count++
	}
	m.notify()
}

func (m *sseMailbox) drain(closed bool) sseBatch {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = m.closed || closed
	batch := m.pending
	m.pending = sseBatch{}
	select {
	case <-m.ready:
	default:
	}
	return batch
}

type sseState struct {
	mailbox      *sseMailbox
	wake         *ui.EffectState[struct{}, int]
	sequence     int
	status       int
	events       []request.SSEEvent
	selected     int
	overflow     bool
	disconnected bool
	failed       bool
	timedOut     bool
}

func newSSEState() *sseState {
	mailbox := &sseMailbox{ready: make(chan struct{}, 1)}
	return &sseState{mailbox: mailbox, wake: ui.NewEffect[struct{}, int](func(ctx context.Context) (struct{}, error) {
		select {
		case <-ctx.Done():
			return struct{}{}, ctx.Err()
		case <-mailbox.ready:
			return struct{}{}, nil
		}
	})}
}

// declareSSE runs at send declaration, when the existing download menu has
// already marked its job after showing the file picker.
func (a *App) declareSSE(c ui.C, t *Tab) *sseMailbox {
	if t.sse == nil {
		if _, ok := a.Svc.Requests.(sseSender); !ok {
			return nil
		}
		if state := a.request.tabs[t.ID]; state != nil && state.downloadJob == t.job {
			return nil
		}
		t.sse = newSSEState()
	}
	stream := t.sse
	if _, _, ok := stream.wake.Take(); ok {
		stream.adopt(stream.mailbox.drain(false))
		stream.sequence++
	}
	ui.Effect(c.Part("sse"), stream.wake, stream.sequence)
	return stream.mailbox
}

func (s *sseState) adopt(batch sseBatch) {
	if batch.status != 0 {
		s.status = batch.status
	}
	s.events = append(s.events, batch.events...)
	s.overflow = s.overflow || batch.overflow
}

func (s *sseState) release() {
	if s.mailbox != nil {
		s.adopt(s.mailbox.drain(true))
		s.mailbox, s.wake = nil, nil
	}
}

func (s *sseState) finish(err error) {
	s.release()
	s.overflow = s.overflow || errors.Is(err, request.ErrSSELimit)
	s.failed = err != nil && !errors.Is(err, request.ErrSSELimit)
	s.timedOut = errors.Is(err, context.DeadlineExceeded)
}

func (a *App) responseSSE(c ui.C, t *Tab) {
	s := t.sse
	labels := a.S.Response
	theme := c.Theme()
	ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
		if t.Response == nil {
			ui.Text(c, strconv.Itoa(s.status)).Bold().Color(a.palette.StatusColor(theme, s.status))
			ui.Text(c, labels.SSEEvents).Bold()
		}
		switch {
		case t.Sending:
			ui.Text(c, labels.SSEConnected).Color(theme.Primary)
			ui.Spacer(c)
			if ui.Button(c, labels.SSEDisconnect).Ghost().Clicked() {
				a.Cancel(t)
			}
		case s.disconnected:
			ui.Text(c, labels.SSEDisconnected).Muted()
		case s.timedOut:
			ui.Text(c, labels.SSETimedOut).Color(a.palette.Warning)
		case s.failed:
			ui.Text(c, labels.SSEFailed).Color(theme.Danger)
		default:
			ui.Text(c, labels.SSEEnded).Muted()
		}
	})
	if s.overflow {
		ui.Paragraph(c, labels.SSEOverflow, 0).Color(a.palette.Warning)
	}
	if len(s.events) == 0 {
		ui.Text(c, labels.SSEEmpty).Muted()
		return
	}
	ui.Row(c).FillWidth().Grow(1).Gap(Space2).Children(func() {
		ui.Scroll(c).Width(Space6 * 6).FillHeight().Scrollbars().Children(func() {
			for index := range s.events {
				label := fmt.Sprintf(labels.SSEEventNumber, index+1)
				button := ui.Button(c.KeyInt(index), label).FillWidth()
				if index != s.selected {
					button.Ghost()
				}
				if button.Clicked() {
					s.selected = index
				}
			}
		})
		ui.Divider(c).Vertical()
		event := s.events[s.selected]
		ui.Scroll(c.KeyInt(s.selected)).Grow(1).FillHeight().Scrollbars().Children(func() {
			ui.Column(c).FillWidth().Gap(Space2).Children(func() {
				ui.Paragraph(c, fmt.Sprintf(labels.SSEEventType, event.Type), 0).FillWidth().Muted()
				if event.ID != "" {
					ui.Paragraph(c, fmt.Sprintf(labels.SSEEventID, event.ID), 0).FillWidth().Muted()
				}
				ui.Paragraph(c, event.Data, 0).FillWidth().Mono()
			})
		})
	})
}
