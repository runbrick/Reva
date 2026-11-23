package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	demoUser      = "demo"
	demoPassword  = "reva123"
	demoToken     = "reva-demo-token"
	demoAPIKey    = "reva-demo-key"
	sessionCookie = "reva_demo_session"
	maxBody       = 8 << 20
)

func main() {
	port := flag.Int("port", 8787, "local HTTP port")
	flag.Parse()
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(*port)))
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("Reva test service at http://%s (GET /health)", listener.Addr())
	log.Fatal(server.Serve(listener))
}

func newHandler() http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /health", health)
	routes.HandleFunc("POST /auth/login", login)
	routes.HandleFunc("GET /auth/bearer", bearer)
	routes.HandleFunc("GET /auth/cookie", cookie)
	routes.HandleFunc("GET /auth/basic", basic)
	routes.HandleFunc("GET /auth/key", apiKey)
	routes.HandleFunc("/echo/{id}", echo)
	routes.HandleFunc("HEAD /echo/{id}", func(output http.ResponseWriter, req *http.Request) {
		output.Header().Set("X-Echo-Id", url.PathEscape(req.PathValue("id")))
		output.WriteHeader(http.StatusOK)
	})
	routes.HandleFunc("OPTIONS /echo/{id}", func(output http.ResponseWriter, _ *http.Request) {
		output.Header().Set("Allow", "GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE")
		output.WriteHeader(http.StatusNoContent)
	})
	routes.HandleFunc("POST /upload", multipartUpload)
	routes.HandleFunc("POST /upload/binary", binaryUpload)
	routes.HandleFunc("GET /download", download)
	routes.HandleFunc("GET /formats/{format}", formats)
	routes.HandleFunc("GET /status/{code}", status)
	routes.HandleFunc("GET /redirect", func(output http.ResponseWriter, req *http.Request) {
		http.Redirect(output, req, "/health", http.StatusFound)
	})
	routes.HandleFunc("GET /delay", delay)
	routes.HandleFunc("GET /events", events)
	return routes
}

func respond(output http.ResponseWriter, code int, data any) {
	output.Header().Set("Content-Type", "application/json; charset=utf-8")
	output.WriteHeader(code)
	_ = json.NewEncoder(output).Encode(data)
}

func problem(output http.ResponseWriter, code int, message string) {
	respond(output, code, map[string]string{"error": message})
}

func bodyError(output http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		problem(output, http.StatusRequestEntityTooLarge, "request exceeds 8 MiB")
		return
	}
	problem(output, http.StatusBadRequest, "invalid request body")
}

func mediaType(req *http.Request) string {
	value, _, _ := mime.ParseMediaType(req.Header.Get("Content-Type"))
	return value
}

func health(output http.ResponseWriter, _ *http.Request) {
	respond(output, http.StatusOK, map[string]string{"status": "ok"})
}

func login(output http.ResponseWriter, req *http.Request) {
	if mediaType(req) != "application/json" {
		problem(output, http.StatusUnsupportedMediaType, "send application/json")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(output, req.Body, maxBody))
	if err != nil {
		bodyError(output, err)
		return
	}
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(data, &credentials); err != nil || credentials.Username == "" || credentials.Password == "" {
		problem(output, http.StatusBadRequest, "send username and password")
		return
	}
	if credentials.Username != demoUser || credentials.Password != demoPassword {
		problem(output, http.StatusUnauthorized, "invalid credentials")
		return
	}
	http.SetCookie(output, &http.Cookie{Name: sessionCookie, Value: demoToken, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
	output.Header().Set("Cache-Control", "no-store")
	respond(output, http.StatusOK, map[string]string{"access_token": demoToken, "token_type": "Bearer"})
}

func bearer(output http.ResponseWriter, req *http.Request) {
	scheme, value, _ := strings.Cut(req.Header.Get("Authorization"), " ")
	if !strings.EqualFold(scheme, "Bearer") || value != demoToken {
		output.Header().Set("WWW-Authenticate", `Bearer realm="Reva demo"`)
		problem(output, http.StatusUnauthorized, "valid bearer token required")
		return
	}
	respond(output, http.StatusOK, map[string]string{"auth": "bearer", "user": demoUser})
}

func cookie(output http.ResponseWriter, req *http.Request) {
	session, err := req.Cookie(sessionCookie)
	if err != nil || session.Value != demoToken {
		problem(output, http.StatusUnauthorized, "session cookie required")
		return
	}
	respond(output, http.StatusOK, map[string]string{"auth": "cookie", "user": demoUser})
}

func basic(output http.ResponseWriter, req *http.Request) {
	username, password, valid := req.BasicAuth()
	if !valid || username != demoUser || password != demoPassword {
		output.Header().Set("WWW-Authenticate", `Basic realm="Reva demo"`)
		problem(output, http.StatusUnauthorized, "valid basic credentials required")
		return
	}
	respond(output, http.StatusOK, map[string]string{"auth": "basic", "user": demoUser})
}

func apiKey(output http.ResponseWriter, req *http.Request) {
	if req.Header.Get("X-API-Key") != demoAPIKey && req.URL.Query().Get("api_key") != demoAPIKey {
		problem(output, http.StatusUnauthorized, "valid API key required")
		return
	}
	respond(output, http.StatusOK, map[string]string{"auth": "api-key", "user": demoUser})
}

func echo(output http.ResponseWriter, req *http.Request) {
	query, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		problem(output, http.StatusBadRequest, "invalid query parameters")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(output, req.Body, maxBody))
	if err != nil {
		bodyError(output, err)
		return
	}
	response := map[string]any{"id": req.PathValue("id"), "method": req.Method, "query": query, "headers": req.Header, "bytes": len(data)}
	if len(data) > 0 {
		switch mediaType(req) {
		case "application/json":
			if !json.Valid(data) {
				problem(output, http.StatusBadRequest, "invalid JSON")
				return
			}
			response["json"] = json.RawMessage(data)
		case "application/x-www-form-urlencoded":
			form, err := url.ParseQuery(string(data))
			if err != nil {
				problem(output, http.StatusBadRequest, "invalid form fields")
				return
			}
			response["form"] = form
		default:
			response["text"] = string(data)
		}
	}
	respond(output, http.StatusOK, response)
}

type uploadedFile struct {
	Filename string `json:"filename"`
	Bytes    int64  `json:"bytes"`
	SHA256   string `json:"sha256"`
}

func multipartUpload(output http.ResponseWriter, req *http.Request) {
	if mediaType(req) != "multipart/form-data" {
		problem(output, http.StatusUnsupportedMediaType, "send multipart/form-data")
		return
	}
	req.Body = http.MaxBytesReader(output, req.Body, maxBody)
	err := req.ParseMultipartForm(maxBody)
	if req.MultipartForm != nil {
		defer req.MultipartForm.RemoveAll()
	}
	if err != nil {
		bodyError(output, err)
		return
	}
	if _, err := io.Copy(io.Discard, req.Body); err != nil {
		bodyError(output, err)
		return
	}
	files := make(map[string][]uploadedFile)
	for field, headers := range req.MultipartForm.File {
		for _, header := range headers {
			file, err := header.Open()
			if err != nil {
				bodyError(output, err)
				return
			}
			info, err := fileInfo(file, header.Filename)
			file.Close()
			if err != nil {
				bodyError(output, err)
				return
			}
			files[field] = append(files[field], info)
		}
	}
	if len(files) == 0 {
		problem(output, http.StatusBadRequest, "at least one file part is required")
		return
	}
	respond(output, http.StatusOK, map[string]any{"fields": req.MultipartForm.Value, "files": files})
}

func fileInfo(reader io.Reader, name string) (uploadedFile, error) {
	hash := sha256.New()
	size, err := io.Copy(hash, reader)
	return uploadedFile{Filename: name, Bytes: size, SHA256: fmt.Sprintf("%x", hash.Sum(nil))}, err
}

func binaryUpload(output http.ResponseWriter, req *http.Request) {
	if mediaType(req) != "application/octet-stream" {
		problem(output, http.StatusUnsupportedMediaType, "send application/octet-stream")
		return
	}
	info, err := fileInfo(http.MaxBytesReader(output, req.Body, maxBody), "")
	if err != nil {
		bodyError(output, err)
		return
	}
	respond(output, http.StatusOK, map[string]any{"bytes": info.Bytes, "sha256": info.SHA256})
}

func download(output http.ResponseWriter, req *http.Request) {
	output.Header().Set("Content-Type", "application/octet-stream")
	output.Header().Set("Content-Disposition", `attachment; filename="reva-sample.txt"`)
	http.ServeContent(output, req, "reva-sample.txt", time.Time{}, strings.NewReader("Reva 下载测试\n"))
}

func formats(output http.ResponseWriter, req *http.Request) {
	var contentType, body string
	switch req.PathValue("format") {
	case "json":
		contentType, body = "application/json", `{"message":"你好，Reva","items":[{"id":1,"active":true},{"id":2,"active":false}],"extra":null}`+"\n"
	case "xml":
		contentType, body = "application/xml", "<?xml version=\"1.0\"?>\n<response><message>你好，Reva</message></response>\n"
	case "html":
		contentType, body = "text/html", "<!doctype html><html lang=\"zh-CN\"><meta charset=\"utf-8\"><title>Reva</title><h1>HTML 响应</h1></html>\n"
	case "text":
		contentType, body = "text/plain", "Reva 文本测试\n第二行\n"
	case "csv":
		contentType, body = "text/csv", "id,name\n1,张三\n2,\"Doe, Jane\"\n"
	case "gzip":
		var compressed bytes.Buffer
		writer := gzip.NewWriter(&compressed)
		_, _ = io.WriteString(writer, `{"encoding":"gzip","message":"你好，Reva"}`+"\n")
		_ = writer.Close()
		output.Header().Set("Content-Type", "application/json; charset=utf-8")
		output.Header().Set("Content-Encoding", "gzip")
		_, _ = output.Write(compressed.Bytes())
		return
	default:
		problem(output, http.StatusNotFound, "unknown response format")
		return
	}
	output.Header().Set("Content-Type", contentType+"; charset=utf-8")
	_, _ = io.WriteString(output, body)
}

func status(output http.ResponseWriter, req *http.Request) {
	code, _ := strconv.Atoi(req.PathValue("code"))
	switch code {
	case http.StatusNoContent:
		output.WriteHeader(code)
	case http.StatusTooManyRequests:
		output.Header().Set("Retry-After", "2")
		fallthrough
	case http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError:
		respond(output, code, map[string]any{"status": code, "message": http.StatusText(code)})
	default:
		problem(output, http.StatusBadRequest, "use status 204, 400, 404, 429 or 500")
	}
}

func delay(output http.ResponseWriter, req *http.Request) {
	value := req.URL.Query().Get("ms")
	if value == "" {
		value = "1500"
	}
	ms, err := strconv.Atoi(value)
	if err != nil || ms < 0 || ms > 10000 {
		problem(output, http.StatusBadRequest, "ms must be between 0 and 10000")
		return
	}
	if pause(req.Context(), time.Duration(ms)*time.Millisecond) {
		respond(output, http.StatusOK, map[string]int{"delay_ms": ms})
	}
}

func events(output http.ResponseWriter, req *http.Request) {
	flusher, ok := output.(http.Flusher)
	if !ok {
		problem(output, http.StatusInternalServerError, "streaming unavailable")
		return
	}
	output.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	output.Header().Set("Cache-Control", "no-cache")
	for index, event := range []string{"progress", "progress", "done"} {
		if index > 0 && !pause(req.Context(), 500*time.Millisecond) {
			return
		}
		if _, err := fmt.Fprintf(output, "id: %d\nevent: %s\ndata: {\"step\":%d}\n\n", index+1, event, index+1); err != nil {
			return
		}
		flusher.Flush()
	}
}

func pause(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
