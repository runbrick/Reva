package request

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"strings"
	"unicode/utf8"

	"github.com/runbrick/reva/internal/model"
)

const (
	maxSSELineBytes  = 16 << 10
	maxSSEEventBytes = 32 << 10
	maxSSERawBytes   = 128 << 10
)

// ErrSSELimit ends inspection when an SSE line or event exceeds its byte limit.
var ErrSSELimit = errors.New("SSE line or event exceeds inspection limits")

// SSEEvent is a complete dispatch, with the effective event type and last ID.
type SSEEvent struct {
	Type, ID, Data string
}

// SSEObserver receives synchronous callbacks on the sending worker. Event
// enables inspection of text/event-stream responses, and must return promptly.
type SSEObserver struct {
	Opened func(statusCode int)
	Event  func(SSEEvent)
}

// SendSSE follows Send's transport policy and inspects actual event streams.
// For SSE, the final response retains up to 128 KiB of raw body and reports
// bytes read, including on a read error. Other media types behave like Send.
func (s *Service) SendSSE(ctx context.Context, req model.Request, variables []model.KV, observer SSEObserver) (model.Response, error) {
	return s.send(ctx, req, variables, observer)
}

func isSSE(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "text/event-stream"
}

type sseCapture struct {
	reader io.Reader
	limit  int
	body   []byte
	size   int64
}

func (c *sseCapture) Read(buffer []byte) (int, error) {
	count, err := c.reader.Read(buffer)
	c.size += int64(count)
	c.body = append(c.body, buffer[:min(count, c.limit-len(c.body))]...)
	return count, err
}

type sseParser struct {
	data          []byte
	eventType, id string
}

func readSSE(ctx context.Context, body io.Reader, emit func(SSEEvent)) error {
	reader := bufio.NewReader(body)
	var parser sseParser
	var line []byte
	first, afterCR := true, false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		value, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if afterCR && value == '\n' {
			afterCR = false
			continue
		}
		afterCR = value == '\r'
		if !afterCR && value != '\n' {
			if len(line) == maxSSELineBytes {
				return ErrSSELimit
			}
			line = append(line, value)
			continue
		}
		if first {
			line = bytes.TrimPrefix(line, []byte("\uFEFF"))
			first = false
		}
		if err := parser.consume(line, emit); err != nil {
			return err
		}
		line = line[:0]
	}
}

func (p *sseParser) consume(line []byte, emit func(SSEEvent)) error {
	if len(line) == 0 {
		if len(p.data) != 0 {
			kind := p.eventType
			if kind == "" {
				kind = "message"
			}
			if len(kind)+len(p.id)+len(p.data)-1 > maxSSEEventBytes {
				return ErrSSELimit
			}
			emit(SSEEvent{Type: kind, ID: p.id, Data: string(p.data[:len(p.data)-1])})
		}
		p.data, p.eventType = p.data[:0], ""
		return nil
	}
	field, value, _ := bytes.Cut(line, []byte(":"))
	value = bytes.TrimPrefix(value, []byte(" "))
	switch string(field) {
	case "data":
		data := decodeSSE(value)
		if len(p.data)+len(data)+1+len(p.eventType)+len(p.id) > maxSSEEventBytes {
			return ErrSSELimit
		}
		p.data = append(p.data, data...)
		p.data = append(p.data, '\n')
	case "event":
		p.eventType = decodeSSE(value)
	case "id":
		if !bytes.ContainsRune(value, 0) {
			p.id = decodeSSE(value)
		}
	}
	if len(p.data)+len(p.eventType)+len(p.id) > maxSSEEventBytes {
		return ErrSSELimit
	}
	return nil
}

func decodeSSE(value []byte) string {
	if utf8.Valid(value) {
		return string(value)
	}
	var decoded strings.Builder
	decoded.Grow(len(value))
	for len(value) != 0 {
		r, width := utf8.DecodeRune(value)
		if r == utf8.RuneError && width == 1 {
			for width < len(value) && !utf8.FullRune(value[:width+1]) {
				width++
			}
		}
		decoded.WriteRune(r)
		value = value[width:]
	}
	return decoded.String()
}
