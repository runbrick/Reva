// Package request sends HTTP requests using the standard HTTP transport.
package request

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/vars"
)

const (
	DefaultTimeout      = 60 * time.Second
	DefaultMaxBodyBytes = 10 << 20
)

// Service implements service.RequestService. Configure its fields before use;
// Send may then be called concurrently. Nonpositive limits use the defaults.
// A nil Transport clones http.DefaultTransport (including gzip decompression).
// A supplied *http.Transport is a template; other RoundTrippers manage TLS
// themselves. At most two transports (TLS policies) and four clients (TLS and
// redirect policies) are cached; timeouts apply to each send's context.
type Service struct {
	Timeout      time.Duration
	MaxBodyBytes int64
	Transport    http.RoundTripper
	mu           sync.Mutex
	clients      map[clientOptions]*http.Client
	transports   map[bool]http.RoundTripper
}

var _ service.RequestService = (*Service)(nil)

// New returns a service with a 60-second timeout and a 10 MiB response body cap.
func New() *Service {
	return &Service{Timeout: DefaultTimeout, MaxBodyBytes: DefaultMaxBodyBytes}
}

// Send resolves variables, sends req, and reads the complete response while
// retaining at most MaxBodyBytes bytes. Size includes bytes beyond that cap.
func (s *Service) Send(ctx context.Context, req model.Request, variables []model.KV) (model.Response, error) {
	if err := ctx.Err(); err != nil {
		return model.Response{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout(req.Options))
	defer cancel()
	req = vars.Apply(req, variables)
	if err := auth.Validate(req.Auth); err != nil {
		return model.Response{}, err
	}
	u, err := requestURL(req.URL)
	if err != nil {
		return model.Response{}, err
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return model.Response{}, fmt.Errorf("invalid URL query: %w", err)
	}
	// The editor mirrors URL query entries in Params, while importers may
	// populate Params alone. Match each row against one original occurrence
	// so neither shape duplicates entries or loses intentional repetitions.
	type pair struct{ key, value string }
	remaining := make(map[pair]int)
	for key, values := range query {
		for _, value := range values {
			remaining[pair{key, value}]++
		}
	}
	for _, row := range req.Params {
		if row.Disabled {
			continue
		}
		if p := (pair{row.Key, row.Value}); remaining[p] > 0 {
			remaining[p]--
		} else {
			query.Add(row.Key, row.Value)
		}
	}
	if err := auth.ApplyQuery(query, req.Auth); err != nil {
		return model.Response{}, err
	}
	u.RawQuery = query.Encode()

	// Use the same boundary when a 307/308 redirect replays a multipart upload.
	factory := newBodyFactory(req.Body)
	body, contentType, err := factory()
	if err != nil {
		return model.Response{}, err
	}
	if body != nil {
		defer body.Close()
	}
	httpReq, err := http.NewRequestWithContext(ctx, string(req.Method), u.String(), body)
	if err != nil {
		return model.Response{}, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		httpReq.GetBody = func() (io.ReadCloser, error) {
			r, _, err := factory()
			return r, err
		}
	}
	for _, row := range req.Headers {
		if !row.Disabled {
			httpReq.Header.Add(row.Key, row.Value)
		}
	}
	if err := auth.ApplyHeaders(httpReq.Header, req.Auth); err != nil {
		return model.Response{}, err
	}
	setDefaultHeader(httpReq.Header, "User-Agent", "Reva/0.1")
	if contentType != "" {
		setDefaultHeader(httpReq.Header, "Content-Type", contentType)
	}
	// Host is special in net/http: it lives on Request, not in Header.
	if values, ok := httpReq.Header["Host"]; ok && len(values) > 0 {
		httpReq.Host = values[0]
	}

	limit := s.MaxBodyBytes
	if limit <= 0 {
		limit = DefaultMaxBodyBytes
	}
	// Copy only the client value to attach this send's tracing wrapper. The
	// cached underlying transport and its connection pool are always reused.
	client := *s.client(req.Options)
	protectRedirects(&client, req.Auth)
	start := time.Now()
	trace := &sendTrace{base: client.Transport, start: start}
	client.Transport = trace
	resp, err := client.Do(httpReq)
	if err != nil {
		return model.Response{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	downloadStart := time.Now()
	kept, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	size := int64(len(kept))
	if err == nil {
		var discarded int64
		discarded, err = io.Copy(io.Discard, resp.Body)
		size += discarded
	}
	// A local round trip can complete within one Windows clock tick. Preserve
	// a positive measurement even when the clock cannot resolve the interval.
	end := time.Now()
	duration := max(end.Sub(start), time.Nanosecond)
	result := model.Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Proto:      resp.Proto,
		Headers:    responseHeaders(resp.Header),
		Body:       kept,
		Size:       size,
		Truncated:  size > int64(len(kept)),
		Duration:   duration,
		URL:        resp.Request.URL.String(),
		Cookies:    responseCookies(resp),
		Timeline:   trace.timeline(downloadStart, end),
	}
	if err != nil {
		return result, fmt.Errorf("read response body: %w", err)
	}
	return result, nil
}

func requestURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("invalid URL: URL is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.Opaque != "" {
		return nil, fmt.Errorf("invalid HTTP URL %q", raw)
	}
	return u, nil
}

func setDefaultHeader(headers http.Header, key, value string) {
	if _, exists := headers[http.CanonicalHeaderKey(key)]; !exists {
		headers.Set(key, value)
	}
}

// protectRedirects wraps policy on a per-send client copy. net/http copies
// initial headers on every hop, so stripping only the first cross-origin hop
// would leak credentials when a later redirect returns to the initial origin.
func protectRedirects(client *http.Client, cfg model.Auth) {
	previous := client.CheckRedirect
	crossedOrigin := false
	strip := func(req *http.Request) {
		// Client.send can synthesize Basic auth from URL userinfo after the
		// redirect callback. Do not allow it to restore stripped Authorization.
		req.URL.User = nil
		for key := range req.Header {
			if strings.EqualFold(key, "Authorization") ||
				cfg.Type == model.AuthAPIKey && cfg.In != model.APIKeyInQuery && strings.EqualFold(key, cfg.Key) {
				delete(req.Header, key)
			}
		}
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		checkOrigin := func() {
			if len(via) > 0 && !sameOrigin(via[len(via)-1].URL, req.URL) {
				crossedOrigin = true
			}
			if crossedOrigin {
				strip(req)
			}
		}
		checkOrigin()
		var err error
		if previous != nil {
			err = previous(req, via)
		} else if len(via) >= 10 {
			err = fmt.Errorf("stopped after 10 redirects")
		}
		// Preserve custom redirect policy while ensuring it cannot reintroduce
		// protected headers or change the destination across an origin boundary.
		checkOrigin()
		return err
	}
}

func sameOrigin(a, b *url.URL) bool {
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if strings.EqualFold(u.Scheme, "https") {
			return "443"
		}
		return "80"
	}
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}

func responseHeaders(headers http.Header) []model.KV {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var rows []model.KV
	for _, key := range keys {
		for _, value := range headers[key] {
			rows = append(rows, model.KV{Key: key, Value: value})
		}
	}
	return rows
}
