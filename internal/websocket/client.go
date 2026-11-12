// Package websocket implements the client side of RFC 6455 for Reqo sessions.
package websocket

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const maxMessage = 8 << 20

type Conn struct {
	net.Conn
	reader *bufio.Reader
	write  sync.Mutex
	once   sync.Once
}

// Dial negotiates an RFC 6455 connection. Headers can supply authentication
// and cookies, but cannot replace the required upgrade headers.
func Dial(ctx context.Context, rawURL string, headers http.Header, timeout time.Duration, insecureTLS bool) (*Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "ws" && u.Scheme != "wss") || u.Fragment != "" || u.User != nil {
		return nil, errors.New("invalid WebSocket URL")
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	port := "80"
	if u.Scheme == "wss" {
		port = "443"
	}
	if u.Port() != "" {
		port = u.Port()
	}
	netConn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
	if err != nil {
		return nil, err
	}
	connected := false
	defer func() {
		if !connected {
			netConn.Close()
		}
	}()
	stop := context.AfterFunc(ctx, func() { netConn.SetDeadline(time.Now()) })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		netConn.SetDeadline(deadline)
	}
	if u.Scheme == "wss" {
		secure := tls.Client(netConn, &tls.Config{ServerName: u.Hostname(), InsecureSkipVerify: insecureTLS})
		if err = secure.HandshakeContext(ctx); err != nil {
			return nil, err
		}
		netConn = secure
	}
	var nonce [16]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(nonce[:])
	requestURL := *u
	if u.Scheme == "wss" {
		requestURL.Scheme = "https"
	} else {
		requestURL.Scheme = "http"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}
	for name, values := range headers {
		switch strings.ToLower(name) {
		case "host", "connection", "upgrade", "sec-websocket-key", "sec-websocket-version", "sec-websocket-accept", "sec-websocket-extensions":
			return nil, fmt.Errorf("reserved WebSocket header: %s", name)
		}
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", key)
	req.Header.Set("Sec-WebSocket-Version", "13")
	if err = req.Write(netConn); err != nil {
		return nil, err
	}
	reader := bufio.NewReader(netConn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	hash := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	if resp.StatusCode != http.StatusSwitchingProtocols || !strings.EqualFold(resp.Header.Get("Upgrade"), "websocket") || !headerHasToken(resp.Header, "Connection", "upgrade") || resp.Header.Get("Sec-WebSocket-Accept") != base64.StdEncoding.EncodeToString(hash[:]) {
		return nil, fmt.Errorf("WebSocket handshake rejected: %s", resp.Status)
	}
	if err = netConn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	connected = true
	return &Conn{Conn: netConn, reader: reader}, nil
}

func headerHasToken(h http.Header, name, token string) bool {
	for _, value := range h.Values(name) {
		for part := range strings.SplitSeq(value, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

// WriteText sends one masked client text frame.
func (c *Conn) WriteText(text string) error { return c.frame(1, []byte(text)) }

func (c *Conn) frame(opcode byte, data []byte) error {
	if len(data) > maxMessage || opcode >= 8 && len(data) > 125 {
		return errors.New("WebSocket frame too large")
	}
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	frame := make([]byte, 2, 14+len(data))
	frame[0] = 0x80 | opcode
	size := len(data)
	switch {
	case size < 126:
		frame[1] = 0x80 | byte(size)
	case size <= 65535:
		frame[1] = 0x80 | 126
		frame = binary.BigEndian.AppendUint16(frame, uint16(size))
	default:
		frame[1] = 0x80 | 127
		frame = binary.BigEndian.AppendUint64(frame, uint64(size))
	}
	frame = append(frame, mask[:]...)
	for i, value := range data {
		frame = append(frame, value^mask[i%4])
	}
	c.write.Lock()
	defer c.write.Unlock()
	for len(frame) > 0 {
		n, err := c.Conn.Write(frame)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}
	return nil
}

// ReadMessage reads a complete text or binary message, handling control and
// fragmented frames. A remote close returns io.EOF.
func (c *Conn) ReadMessage() (byte, []byte, error) {
	var message []byte
	var kind byte
	for {
		var head [2]byte
		if _, err := io.ReadFull(c.reader, head[:]); err != nil {
			return 0, nil, err
		}
		if head[0]&0x70 != 0 || head[1]&0x80 != 0 {
			return 0, nil, errors.New("invalid server WebSocket frame")
		}
		opcode, final := head[0]&15, head[0]&0x80 != 0
		length := uint64(head[1] & 127)
		if length == 126 {
			var size [2]byte
			if _, err := io.ReadFull(c.reader, size[:]); err != nil {
				return 0, nil, err
			}
			length = uint64(binary.BigEndian.Uint16(size[:]))
		} else if length == 127 {
			var size [8]byte
			if _, err := io.ReadFull(c.reader, size[:]); err != nil {
				return 0, nil, err
			}
			length = binary.BigEndian.Uint64(size[:])
		}
		if length > maxMessage || uint64(len(message))+length > maxMessage || opcode >= 8 && (!final || length > 125) {
			return 0, nil, errors.New("WebSocket message too large or invalid")
		}
		data := make([]byte, int(length))
		if _, err := io.ReadFull(c.reader, data); err != nil {
			return 0, nil, err
		}
		switch opcode {
		case 8:
			_ = c.frame(8, data)
			return 0, nil, io.EOF
		case 9:
			if err := c.frame(10, data); err != nil {
				return 0, nil, err
			}
		case 10:
		case 0:
			if kind == 0 {
				return 0, nil, errors.New("unexpected WebSocket continuation")
			}
			message = append(message, data...)
		case 1, 2:
			if kind != 0 {
				return 0, nil, errors.New("unfinished WebSocket message")
			}
			kind, message = opcode, append(message, data...)
		default:
			return 0, nil, errors.New("unsupported WebSocket frame")
		}
		if final && kind != 0 && opcode < 8 {
			return kind, message, nil
		}
	}
}

// Close sends a normal close frame and releases the socket to unblock readers.
func (c *Conn) Close() error {
	var err error
	c.once.Do(func() {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
		_ = c.frame(8, []byte{3, 232})
		err = c.Conn.Close()
	})
	return err
}
