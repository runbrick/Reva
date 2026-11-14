package importer

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FetchDocument fetches only an explicitly supplied HTTP(S) schema URL. Parse
// remains offline: neither function follows external $refs or reads local files.
// The returned URL includes redirects so relative OpenAPI servers resolve there.
func FetchDocument(ctx context.Context, address string) (string, string, error) {
	valid := func(address string) bool {
		u, err := url.Parse(address)
		return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil && u.Fragment == ""
	}
	address = strings.TrimSpace(address)
	if !valid(address) {
		return "", "", &ParseError{Code: "invalid_source_url", Path: "$"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return "", "", &ParseError{Code: "invalid_source_url", Path: "$"}
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || !valid(req.URL.String()) {
			return &ParseError{Code: "fetch_failed", Path: "$"}
		}
		return nil
	}}
	response, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return "", "", ctx.Err()
		}
		return "", "", &ParseError{Code: "fetch_failed", Path: "$"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", "", &ParseError{Code: "fetch_failed", Path: "$"}
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxInputBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return "", "", ctx.Err()
		}
		return "", "", &ParseError{Code: "fetch_failed", Path: "$"}
	}
	if len(data) > MaxInputBytes {
		return "", "", &ParseError{Code: "input_too_large", Path: "$"}
	}
	return string(data), response.Request.URL.String(), nil
}
