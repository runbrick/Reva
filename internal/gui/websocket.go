package gui

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/runbrick/reqo/internal/gui/icons"
	"github.com/runbrick/reqo/internal/model"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/runbrick/reqo/internal/vars"
	"github.com/runbrick/reqo/internal/websocket"
	"github.com/vivaui/viva/ui"
)

const webSocketMethod model.Method = "WS"

func isWebSocket(r model.Request) bool { return r.Method == webSocketMethod }

type websocketState struct {
	tabs map[int]*websocketTab
}

type websocketTab struct {
	requestID   string
	headers     kvTableState
	conn        *websocket.Conn
	connect     *ui.EffectState[*websocket.Conn, *websocketDial]
	read        *ui.EffectState[websocketMessage, *websocketRead]
	write       *ui.EffectState[struct{}, *websocketWrite]
	dialJob     *websocketDial
	readJob     *websocketRead
	writeJob    *websocketWrite
	connecting  bool
	reading     bool
	writing     bool
	sequence    int
	log         []websocketMessage
	pending     []websocketMessage
	message     string
	errorText   string
	showHeaders bool
}

type websocketDial struct {
	url      string
	headers  http.Header
	timeout  time.Duration
	insecure bool
	adopted  atomic.Bool
}

type websocketRead struct {
	conn      *websocket.Conn
	sequence  int
	truncated string
}

type websocketWrite struct {
	conn *websocket.Conn
	text string
}

type websocketMessage struct {
	text     string
	at       time.Time
	received bool
	binary   bool
}

func (a *App) newWebSocket(n collectionNode) {
	r := model.NewRequest(storage.NewID())
	r.Method, r.Name = webSocketMethod, a.S.Collections.NewWebSocket
	if n.collectionID != "" {
		stored, err := a.Svc.Collections.SaveRequest(n.collectionID, n.id, r)
		if a.collectionError(err) {
			return
		}
		a.collections.closed[a.collections.nodeKey(n)] = false
		a.refreshCollections()
		a.OpenRequest(stored, n.collectionID)
	} else {
		t := a.NewTab()
		t.Request = r
	}
	a.Main, a.Section = PageWorkbench, SectionAPIs
}

func (a *App) websocketTab(t *Tab) *websocketTab {
	if a.websocket.tabs == nil {
		a.websocket.tabs = make(map[int]*websocketTab)
	}
	s := a.websocket.tabs[t.ID]
	if s == nil || s.requestID != t.Request.ID {
		if s != nil && s.conn != nil {
			s.conn.Close()
		}
		s = &websocketTab{requestID: t.Request.ID, message: t.Request.Body.Text}
		s.headers.headers = true
		s.connect = ui.NewEffectKeyed(func(ctx context.Context, job *websocketDial) (*websocket.Conn, error) {
			conn, err := websocket.Dial(ctx, job.url, job.headers, job.timeout, job.insecure)
			if err != nil {
				return nil, err
			}
			context.AfterFunc(ctx, func() {
				if !job.adopted.Load() {
					conn.Close()
				}
			})
			if ctx.Err() != nil {
				conn.Close()
				return nil, ctx.Err()
			}
			return conn, nil
		})
		s.read = ui.NewEffectKeyed(func(ctx context.Context, job *websocketRead) (websocketMessage, error) {
			stop := context.AfterFunc(ctx, func() { job.conn.Close() })
			defer stop()
			kind, data, err := job.conn.ReadMessage()
			if err != nil {
				return websocketMessage{}, err
			}
			message := websocketMessage{at: time.Now(), received: true, binary: kind == 2, text: websocketPreview(string(data), job.truncated)}
			if message.binary {
				message.text = base64.StdEncoding.EncodeToString(data[:min(len(data), 16<<10)])
				if len(data) > 16<<10 {
					message.text += job.truncated
				}
			}
			return message, nil
		})
		s.write = ui.NewEffectKeyed(func(_ context.Context, job *websocketWrite) (struct{}, error) {
			return struct{}{}, job.conn.WriteText(job.text)
		})
		a.websocket.tabs[t.ID] = s
	}
	return s
}

func (s *websocketTab) disconnect() {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	s.connecting, s.reading, s.writing = false, false, false
	s.flushReceived()
}

func (s *websocketTab) flushReceived() {
	for _, message := range s.pending {
		s.append(message)
	}
	s.pending = nil
}

func (s *websocketTab) append(message websocketMessage) {
	if len(s.log) >= 500 {
		copy(s.log, s.log[1:])
		s.log = s.log[:499]
	}
	s.log = append(s.log, message)
}

func websocketPreview(text, truncated string) string {
	if len(text) <= 16<<10 {
		return text
	}
	return strings.ToValidUTF8(text[:16<<10], "") + truncated
}

// Effects are declared for all open tabs, including ones outside the active view.
func (a *App) declareWebSockets(c ui.C) {
	for id, s := range a.websocket.tabs {
		open := false
		for _, t := range a.tabs {
			if t.ID == id && t.Request.ID == s.requestID && isWebSocket(t.Request) {
				open = true
				break
			}
		}
		if !open {
			s.disconnect()
			delete(a.websocket.tabs, id)
			continue
		}
		if s.connecting {
			ui.Effect(c.KeyInt(id), s.connect, s.dialJob)
		}
		if conn, err, ok := s.connect.Take(); ok {
			wasConnecting := s.connecting
			s.connecting = false
			if err != nil {
				if wasConnecting {
					s.errorText = err.Error()
				}
			} else if wasConnecting && s.conn == nil {
				s.dialJob.adopted.Store(true)
				s.conn, s.errorText = conn, ""
			} else {
				conn.Close()
			}
		}
		if s.writing {
			ui.Effect(c.KeyInt(id), s.write, s.writeJob)
			if _, err, ok := s.write.Take(); ok {
				s.writing = false
				if err != nil {
					s.errorText = err.Error()
					s.disconnect()
				} else {
					s.append(websocketMessage{text: websocketPreview(s.writeJob.text, a.S.WebSocket.Truncated), at: time.Now()})
				}
				s.flushReceived()
			}
		}
		if s.conn != nil && !s.reading {
			s.sequence++
			s.readJob = &websocketRead{conn: s.conn, sequence: s.sequence, truncated: a.S.WebSocket.Truncated}
			s.reading = true
		}
		if s.reading {
			ui.Effect(c.KeyInt(id), s.read, s.readJob)
			if message, err, ok := s.read.Take(); ok {
				s.reading = false
				if err != nil {
					if !errors.Is(err, io.EOF) {
						s.errorText = err.Error()
					}
					s.disconnect()
				} else if s.writing {
					s.pending = append(s.pending, message)
				} else {
					s.append(message)
				}
			}
		}
	}
}

func (a *App) websocketConnect(t *Tab, s *websocketTab) {
	r := vars.Apply(t.Request, a.Vars())
	headers := make(http.Header)
	for _, row := range r.Headers {
		if !row.Disabled && strings.TrimSpace(row.Key) != "" {
			headers.Add(row.Key, row.Value)
		}
	}
	options := t.Request.Options.Resolve(a.Svc.Settings.Get())
	s.dialJob = &websocketDial{url: r.URL, headers: headers, timeout: time.Duration(options.TimeoutMs) * time.Millisecond, insecure: options.VerifyTLS == model.ToggleOff}
	s.errorText, s.connecting = "", true
}

func (a *App) websocketPane(c ui.C, t *Tab) {
	s, labels, theme := a.websocketTab(t), a.S.WebSocket, c.Theme()
	ui.Column(c).Fill().Pad(Space4).Gap(Space3).Children(func() {
		ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
			if ui.TextField(c, &t.Request.URL, labels.URL, labels.URLHint).Grow(1).Changed() {
				t.Dirty = true
			}
			if s.conn != nil {
				if ui.Button(c, labels.Disconnect).Icon(icons.X).Clicked() {
					s.disconnect()
				}
			} else if ui.Button(c, labels.Connect).Icon(icons.Activity).Disabled(s.connecting || strings.TrimSpace(t.Request.URL) == "").Clicked() {
				a.websocketConnect(t, s)
			}
			if ui.Button(c, labels.Save).Icon(icons.Save).Ghost().Clicked() {
				c.After(func() { a.RequestSave(t) })
			}
		})
		status := labels.Disconnected
		color := theme.MutedForeground
		if s.connecting {
			status = labels.Connecting
			color = a.palette.Warning
		} else if s.conn != nil {
			status = labels.Connected
			color = theme.Success
		}
		ui.Text(c, status).Color(color).FontSize(theme.Metrics.FontSizeS)
		if s.errorText != "" {
			ui.Wrap(c, labels.Error+": "+s.errorText).Color(theme.Danger)
		}
		if ui.Button(c, labels.Headers).Icon(icons.ChevronDown).Ghost().Clicked() {
			s.showHeaders = !s.showHeaders
		}
		if s.showHeaders && a.kvTable(c, &s.headers, &t.Request.Headers) {
			t.Dirty = true
		}
		ui.Divider(c)
		ui.Row(c).FillWidth().Items(ui.Center).Children(func() {
			ui.Text(c, labels.Messages).Bold().Grow(1)
			if ui.Button(c, labels.Clear).Ghost().Disabled(len(s.log) == 0).Clicked() {
				s.log = nil
			}
		})
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(Space2).Children(func() {
				if len(s.log) == 0 {
					ui.Text(c, labels.Empty).Muted()
				}
				for i, message := range s.log {
					mc := c.KeyInt(i)
					caption := labels.Sent
					if message.received {
						caption = labels.Received
					}
					if message.binary {
						caption += " / " + labels.Binary
					}
					ui.Column(mc).FillWidth().Pad(Space2).Gap(Space1).Background(theme.Muted).Radius(theme.Metrics.RadiusControl).Children(func() {
						ui.Text(mc, caption+"  "+message.at.Format("15:04:05")).Muted().FontSize(theme.Metrics.FontSizeS)
						ui.Wrap(mc, message.text)
					})
				}
			})
		})
		ui.Row(c).FillWidth().Gap(Space2).Items(ui.Center).Children(func() {
			if ui.TextField(c, &s.message, labels.Message, labels.Message).Grow(1).Submitted() && s.conn != nil && !s.writing {
				a.websocketSend(s)
			}
			if ui.Button(c, labels.Send).Icon(icons.Send).Disabled(s.conn == nil || s.writing || s.message == "").Clicked() {
				a.websocketSend(s)
			}
		})
	})
}

func (a *App) websocketSend(s *websocketTab) {
	s.writeJob = &websocketWrite{conn: s.conn, text: s.message}
	s.writing = true
	s.message = ""
}
