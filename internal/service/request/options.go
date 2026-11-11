package request

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/runbrick/reqo/internal/model"
)

type clientOptions struct {
	timeout     time.Duration
	noRedirects bool
	insecureTLS bool
}

func (s *Service) client(options model.RequestOptions) *http.Client {
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if options.TimeoutMs > 0 {
		// Saturate rather than overflowing into a nonpositive (unbounded) timeout.
		timeout = time.Duration(min(int64(options.TimeoutMs), int64((1<<63-1)/time.Millisecond))) * time.Millisecond
	}
	key := clientOptions{timeout, options.FollowRedirects == model.ToggleOff, options.VerifyTLS == model.ToggleOff}
	s.mu.Lock()
	defer s.mu.Unlock()
	if client := s.clients[key]; client != nil {
		return client
	}
	transport := s.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if template, ok := transport.(*http.Transport); ok {
		cloned := template.Clone()
		if cloned.TLSClientConfig == nil {
			cloned.TLSClientConfig = &tls.Config{}
		} else {
			cloned.TLSClientConfig = cloned.TLSClientConfig.Clone()
		}
		cloned.TLSClientConfig.InsecureSkipVerify = key.insecureTLS
		transport = cloned
	}
	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if key.noRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) > 10 {
				return fmt.Errorf("stopped after 10 redirects")
			}
			return nil
		},
	}
	if s.clients == nil {
		s.clients = make(map[clientOptions]*http.Client)
	}
	s.clients[key] = client
	return client
}

func responseCookies(response *http.Response) []model.Cookie {
	var cookies []model.Cookie
	for _, cookie := range response.Cookies() {
		sameSite := ""
		switch cookie.SameSite {
		case http.SameSiteLaxMode:
			sameSite = "Lax"
		case http.SameSiteStrictMode:
			sameSite = "Strict"
		case http.SameSiteNoneMode:
			sameSite = "None"
		}
		cookies = append(cookies, model.Cookie{
			Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain,
			Path: cookie.Path, Expires: cookie.Expires, MaxAge: cookie.MaxAge,
			HTTPOnly: cookie.HttpOnly, Secure: cookie.Secure, SameSite: sameSite,
		})
	}
	return cookies
}
