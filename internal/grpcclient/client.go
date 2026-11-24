package grpcclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/runbrick/reva/internal/model"
)

const (
	MaxMessageBytes               = 1 << 20
	NoTimeout       time.Duration = -1
)

var (
	ErrURL         = errors.New("invalid gRPC URL")
	ErrHex         = errors.New("invalid protobuf hex")
	ErrLimit       = errors.New("gRPC message limit exceeded")
	ErrMetadata    = errors.New("invalid gRPC metadata")
	ErrTransport   = errors.New("gRPC transport failed")
	ErrHTTPStatus  = errors.New("unexpected HTTP status for gRPC")
	ErrProtocol    = errors.New("invalid unary gRPC response")
	ErrCompression = errors.New("compressed gRPC messages are unsupported")
)

var methodPath = regexp.MustCompile(`^/[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*/[A-Za-z_][A-Za-z0-9_]*$`)

type Request struct {
	URL, Hex    string
	Metadata    []model.KV
	Timeout     time.Duration
	InsecureTLS bool
}

type Result struct {
	HTTPStatus    int
	Status        int
	StatusMessage string
	Message       []byte
	Duration      time.Duration
}

func Call(ctx context.Context, input Request) (Result, error) {
	start := time.Now()
	if input.Timeout != NoTimeout {
		if input.Timeout <= 0 {
			input.Timeout = 30 * time.Second
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, input.Timeout)
		defer cancel()
	}
	result, err := call(ctx, input)
	result.Duration = time.Since(start)
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, err
}

func call(ctx context.Context, input Request) (Result, error) {
	var result Result
	endpoint, err := url.Parse(strings.TrimSpace(input.URL))
	if err != nil || (endpoint.Scheme != "https" && endpoint.Scheme != "http") || endpoint.Hostname() == "" ||
		endpoint.User != nil || endpoint.ForceQuery || endpoint.RawQuery != "" || endpoint.Fragment != "" ||
		endpoint.EscapedPath() != endpoint.Path || !methodPath.MatchString(endpoint.Path) {
		return result, ErrURL
	}
	frame, err := requestFrame(input.Hex)
	if err != nil {
		return result, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(frame))
	if err != nil {
		return result, ErrURL
	}
	if err := addMetadata(req.Header, input.Metadata); err != nil {
		return result, err
	}
	req.Header.Set("Content-Type", "application/grpc")
	req.Header.Set("TE", "trailers")
	if deadline, ok := ctx.Deadline(); ok {
		req.Header.Set("Grpc-Timeout", timeoutHeader(time.Until(deadline)))
	}
	protocols := new(http.Protocols)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(true)
	transport := &http.Transport{
		Protocols:              protocols,
		DisableCompression:     true,
		MaxResponseHeaderBytes: 64 << 10,
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: input.InsecureTLS,
			VerifyConnection: func(state tls.ConnectionState) error {
				if state.NegotiatedProtocol != "h2" {
					return ErrTransport
				}
				return nil
			},
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Do(req)
	if err != nil {
		return result, ErrTransport
	}
	defer resp.Body.Close()
	result.HTTPStatus = resp.StatusCode
	if resp.ProtoMajor != 2 {
		return result, ErrProtocol
	}
	if resp.StatusCode != http.StatusOK {
		return result, ErrHTTPStatus
	}
	contentType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || (contentType != "application/grpc" && !strings.HasPrefix(contentType, "application/grpc+")) {
		return result, ErrProtocol
	}
	return readResponse(result, resp)
}

func timeoutHeader(remaining time.Duration) string {
	for _, unit := range []struct {
		scale time.Duration
		code  string
	}{{time.Millisecond, "m"}, {time.Second, "S"}, {time.Minute, "M"}, {time.Hour, "H"}} {
		value := max(int64(0), int64(remaining/unit.scale))
		if remaining%unit.scale > 0 {
			value++
		}
		if value <= 99999999 {
			return strconv.FormatInt(value, 10) + unit.code
		}
	}
	return "99999999H"
}

func requestFrame(text string) ([]byte, error) {
	frame := make([]byte, 5, 5+min(len(text)/2, MaxMessageBytes))
	var high byte
	var pending bool
	for pos := 0; pos < len(text); pos++ {
		value := text[pos]
		switch {
		case value >= '0' && value <= '9':
			value -= '0'
		case value >= 'a' && value <= 'f':
			value = value - 'a' + 10
		case value >= 'A' && value <= 'F':
			value = value - 'A' + 10
		case value == ' ' || value == '\t' || value == '\r' || value == '\n':
			continue
		default:
			return nil, ErrHex
		}
		if pending {
			if len(frame) == MaxMessageBytes+5 {
				return nil, ErrLimit
			}
			frame = append(frame, high<<4|value)
		} else {
			high = value
		}
		pending = !pending
	}
	if pending {
		return nil, ErrHex
	}
	binary.BigEndian.PutUint32(frame[1:], uint32(len(frame)-5))
	return frame, nil
}

func addMetadata(headers http.Header, rows []model.KV) error {
	for _, row := range rows {
		name := strings.ToLower(strings.TrimSpace(row.Key))
		if row.Disabled || name == "" && row.Value == "" {
			continue
		}
		if name == "" || strings.HasPrefix(name, "grpc-") || strings.ContainsFunc(name, func(char rune) bool {
			return !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-' || char == '_' || char == '.')
		}) {
			return ErrMetadata
		}
		switch name {
		case "host", "content-type", "content-length", "te", "trailer", "connection", "proxy-connection", "keep-alive", "transfer-encoding", "upgrade":
			return ErrMetadata
		}
		value := row.Value
		if strings.ContainsFunc(value, func(char rune) bool { return char < 0x20 || char > 0x7e }) {
			return ErrMetadata
		}
		if strings.HasSuffix(name, "-bin") && strings.HasSuffix(value, "=") {
			if _, err := base64.StdEncoding.Strict().DecodeString(value); err != nil {
				return ErrMetadata
			}
			value = strings.TrimRight(value, "=")
		} else if strings.HasSuffix(name, "-bin") {
			if _, err := base64.RawStdEncoding.Strict().DecodeString(value); err != nil {
				return ErrMetadata
			}
		}
		headers.Add(name, value)
	}
	return nil
}

func readResponse(result Result, resp *http.Response) (Result, error) {
	message, err := responseFrame(resp.Body)
	if err != nil {
		return result, err
	}
	statusFields := resp.Trailer
	if len(resp.Header.Values("Grpc-Status")) > 0 {
		if message != nil || len(resp.Trailer.Values("Grpc-Status")) != 0 {
			return result, ErrProtocol
		}
		statusFields = resp.Header
	}
	status := statusFields.Values("Grpc-Status")
	if len(status) != 1 {
		return result, ErrProtocol
	}
	code, err := strconv.ParseUint(status[0], 10, 31)
	if err != nil || code == 0 && message == nil {
		return result, ErrProtocol
	}
	result.Status, result.Message = int(code), message
	result.StatusMessage = statusFields.Get("Grpc-Message")
	if decoded, err := url.PathUnescape(result.StatusMessage); err == nil {
		result.StatusMessage = strings.ToValidUTF8(decoded, "\ufffd")
	}
	return result, nil
}

func responseFrame(reader io.Reader) ([]byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(reader, header[:]); err == io.EOF {
		return nil, nil
	} else if err != nil {
		return nil, ErrProtocol
	}
	switch header[0] {
	case 0:
	case 1:
		return nil, ErrCompression
	default:
		return nil, ErrProtocol
	}
	size := binary.BigEndian.Uint32(header[1:])
	if size > MaxMessageBytes {
		return nil, ErrLimit
	}
	message := make([]byte, size)
	if _, err := io.ReadFull(reader, message); err != nil {
		return nil, ErrProtocol
	}
	if _, err := io.ReadFull(reader, header[:1]); err != io.EOF {
		return nil, ErrProtocol
	}
	return message, nil
}
