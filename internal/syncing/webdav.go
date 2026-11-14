package syncing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
)

type WebDAVConfig struct {
	URL, User, Password, File string
}

type WebDAV struct {
	base   *url.URL
	config WebDAVConfig
	client *http.Client
}

func NewWebDAV(config WebDAVConfig) (*WebDAV, error) {
	u, err := url.Parse(strings.TrimSpace(config.URL))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid WebDAV address")
	}
	if u.Scheme != "https" {
		ip := net.ParseIP(u.Hostname())
		if u.Scheme != "http" || u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return nil, errors.New("WebDAV credentials require HTTPS outside localhost")
		}
	}
	if config.File == "" {
		config.File = "reva/backup.revabak"
	}
	if strings.HasPrefix(config.File, "/") || strings.Contains(config.File, "..") || strings.ContainsAny(config.File, "\\\r\n\x00") || path.Clean(config.File) == "." {
		return nil, errors.New("invalid backup file path")
	}
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Host != via[0].URL.Host || req.URL.Scheme != via[0].URL.Scheme {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	return &WebDAV{base: u, config: config, client: client}, nil
}

func (w *WebDAV) address(file string) string {
	u := *w.base
	u.Path = strings.TrimSuffix(w.base.Path, "/") + "/" + file
	return u.String()
}

func (w *WebDAV) request(ctx context.Context, method, file string, data []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, w.address(file), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(w.config.User, w.config.Password)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		return nil, errors.New("WebDAV rejected the credentials")
	}
	return resp, nil
}

func (w *WebDAV) Get(ctx context.Context) ([]byte, string, error) {
	resp, err := w.request(ctx, http.MethodGet, w.config.File, nil, nil)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, "", os.ErrNotExist
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("WebDAV GET: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxBytes+1))
	if len(data) > MaxBytes {
		return nil, "", errors.New("remote backup exceeds size limit")
	}
	return data, resp.Header.Get("ETag"), err
}

func (w *WebDAV) Put(ctx context.Context, data []byte, previous string) (string, error) {
	if len(data) > MaxBytes {
		return "", errors.New("backup exceeds size limit")
	}
	parts := strings.Split(w.config.File, "/")
	for n := 1; n < len(parts); n++ {
		resp, err := w.request(ctx, "MKCOL", strings.Join(parts[:n], "/"), nil, nil)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("WebDAV folder %s: %s", strings.Join(parts[:n], "/"), resp.Status)
		}
	}
	if previous == "" {
		// If a server does not supply ETags, refusing replacement is safer
		// than losing another device's data.
		resp, err := w.request(ctx, http.MethodGet, w.config.File, nil, nil)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return "", ErrConflict
		}
		if resp.StatusCode != http.StatusNotFound {
			return "", fmt.Errorf("WebDAV GET: %s", resp.Status)
		}
	}
	headers := map[string]string{"Content-Type": "application/octet-stream"}
	if previous == "" {
		headers["If-None-Match"] = "*"
	} else {
		headers["If-Match"] = previous
	}
	resp, err := w.request(ctx, http.MethodPut, w.config.File, data, headers)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusPreconditionFailed {
		return "", ErrConflict
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("WebDAV PUT: %s", resp.Status)
	}
	return resp.Header.Get("ETag"), nil
}
