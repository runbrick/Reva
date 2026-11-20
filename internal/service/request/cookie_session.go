package request

import (
	"context"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
)

type httpOrigin struct {
	scheme, host, port string
}

func originOf(u *url.URL) httpOrigin {
	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return httpOrigin{scheme: scheme, host: strings.ToLower(u.Hostname()), port: port}
}

// CookieSession holds in-memory cookies isolated by scheme, host and port.
type CookieSession struct {
	mu   sync.Mutex
	jars map[httpOrigin]*cookiejar.Jar
}

func NewCookieSession() *CookieSession { return &CookieSession{} }

type cookieSessionKey struct{}

func WithCookieSession(ctx context.Context, session *CookieSession) context.Context {
	if session == nil {
		return ctx
	}
	return context.WithValue(ctx, cookieSessionKey{}, session)
}

func (s *CookieSession) jarFor(u *url.URL) *cookiejar.Jar {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jars == nil {
		s.jars = make(map[httpOrigin]*cookiejar.Jar)
	}
	origin := originOf(u)
	if jar := s.jars[origin]; jar != nil {
		return jar
	}
	jar, _ := cookiejar.New(nil)
	s.jars[origin] = jar
	return jar
}
