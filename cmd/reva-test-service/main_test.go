package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/importer"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service/request"
)

func TestImportedCollectionSendsThroughReva(t *testing.T) {
	data, err := os.ReadFile("reva-test.postman_collection.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := importer.Parse(string(data))
	if err != nil || result.Collection == nil {
		t.Fatalf("import failed: %v", err)
	}
	for _, warning := range result.Warnings {
		if warning.Code != "local_file_references" {
			t.Errorf("unexpected import warning: %+v", warning)
		}
	}

	server := httptest.NewServer(newHandler())
	defer server.Close()
	client := request.New()
	ctx := request.WithCookieSession(t.Context(), request.NewCookieSession())
	file := filepath.Join(t.TempDir(), "demo.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	expected := map[string]struct {
		status int
		body   string
	}{
		"健康检查":         {200, `"status":"ok"`},
		"错误密码":         {401, `"error":"invalid credentials"`},
		"尚未登录":         {401, `"error":"session cookie required"`},
		"登录":           {200, `"access_token":"reva-demo-token"`},
		"Bearer 鉴权":    {200, `"auth":"bearer"`},
		"Cookie 会话":    {200, `"auth":"cookie"`},
		"Basic 鉴权":     {200, `"auth":"basic"`},
		"API Key 请求头":  {200, `"auth":"api-key"`},
		"API Key 查询参数": {200, `"auth":"api-key"`},
		"路径、查询和请求头":    {200, `"id":"订单 42"`},
		"POST JSON":    {200, `"json":{"name":"测试","enabled":true,"count":2}`},
		"PUT 表单":       {200, `"form":{"tag":["one","two"]}`},
		"PATCH XML":    {200, `"text":"\u003cname\u003e中文\u003c/name\u003e"`},
		"DELETE":       {200, `"method":"DELETE"`},
		"HEAD":         {200, ""},
		"OPTIONS":      {204, ""},
		"Multipart 上传": {200, `"filename":"demo.txt","bytes":5`},
		"Binary 上传":    {200, `"bytes":5`},
		"下载附件":         {200, "Reva 下载测试\n"},
		"JSON 响应":      {200, `"items":[`},
		"XML 响应":       {200, "<message>"},
		"HTML 响应":      {200, "<h1>"},
		"纯文本响应":        {200, "Reva 文本测试\n"},
		"CSV 响应":       {200, "id,name\n"},
		"Gzip JSON":    {200, `"encoding":"gzip"`},
		"SSE 事件流":      {200, "event: progress"},
		"204 无内容":      {204, ""},
		"400 请求错误":     {400, `"status":400`},
		"404 未找到":      {404, `"status":404`},
		"429 请求限流":     {429, `"status":429`},
		"500 服务端错误":    {500, `"status":500`},
		"302 重定向":      {200, `"status":"ok"`},
		"延迟响应":         {200, `"delay_ms":1500`},
	}
	variables := result.Collection.Variables
	if len(variables) != 1 || variables[0].Key != "baseUrl" || variables[0].Value != "http://127.0.0.1:8787" {
		t.Fatalf("unexpected collection variables: %+v", variables)
	}
	variables[0].Value = server.URL
	model.WalkRequests(result.Collection.Items, func(req *model.Request) {
		want, ok := expected[req.Name]
		if !ok {
			t.Errorf("unexpected request %q", req.Name)
			return
		}
		delete(expected, req.Name)
		t.Run(req.Name, func(t *testing.T) {
			resolvedAuth, _, err := auth.Resolve(*result.Collection, req.ID, req.Auth)
			if err != nil {
				t.Fatal(err)
			}
			req.Auth = resolvedAuth
			for index := range req.Body.Form {
				if req.Body.Form[index].File {
					req.Body.Form[index].Value = file
				}
			}
			if req.Body.Mode == model.BodyBinary {
				req.Body.FilePath = file
			}
			var events []request.SSEEvent
			resp, err := client.SendSSE(ctx, *req, variables, request.SSEObserver{
				Event: func(event request.SSEEvent) { events = append(events, event) },
			})
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != want.status || !strings.Contains(string(resp.Body), want.body) || (want.body == "" && len(resp.Body) != 0) {
				t.Fatalf("status = %d, body = %q; want %d containing %q", resp.StatusCode, resp.Body, want.status, want.body)
			}
			switch req.Name {
			case "路径、查询和请求头":
				var echo struct {
					ID      string
					Query   map[string][]string
					Headers http.Header
				}
				if err := json.Unmarshal(resp.Body, &echo); err != nil || echo.ID != "订单 42" || len(echo.Query["tag"]) != 2 || echo.Query["tag"][0] != "中文" || echo.Query["tag"][1] != "a+b" || len(echo.Query["empty"]) != 1 || echo.Query["empty"][0] != "" || echo.Headers.Get("X-Demo") != "reva" {
					t.Fatalf("unexpected echo: %+v, %v", echo, err)
				}
			case "登录":
				if len(resp.Cookies) != 1 || !resp.Cookies[0].HTTPOnly {
					t.Fatalf("unexpected login cookies: %+v", resp.Cookies)
				}
			case "SSE 事件流":
				if len(events) != 3 || events[0].ID != "1" || events[2].Type != "done" {
					t.Fatalf("unexpected SSE events: %+v", events)
				}
			}
		})
	})
	if len(expected) != 0 {
		t.Errorf("missing requests: %v", expected)
	}
}

func TestInvalidRequestsReturnExpectedStatus(t *testing.T) {
	handler := newHandler()
	for _, test := range []struct {
		name, method, path, contentType string
		body                            io.Reader
		status                          int
	}{
		{"missing bearer", "GET", "/auth/bearer", "", nil, 401},
		{"bad JSON", "POST", "/echo/test", "application/json", strings.NewReader("{"), 400},
		{"bad query", "GET", "/echo/test?x=%ff%", "", nil, 400},
		{"missing file", "POST", "/upload", "multipart/form-data; boundary=test", strings.NewReader("--test--\r\n"), 400},
		{"wrong upload type", "POST", "/upload", "text/plain", strings.NewReader("hello"), 415},
		{"large binary", "POST", "/upload/binary", "application/octet-stream", strings.NewReader(strings.Repeat("x", (8<<20)+1)), 413},
		{"bad status", "GET", "/status/999", "", nil, 400},
		{"bad delay", "GET", "/delay?ms=10001", "", nil, 400},
		{"wrong method", "PUT", "/auth/login", "", nil, 405},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.path, test.body)
			req.Header.Set("Content-Type", test.contentType)
			resp := httptest.NewRecorder()
			handler.ServeHTTP(resp, req)
			if resp.Code != test.status {
				t.Fatalf("status = %d, body = %.250q; want %d", resp.Code, resp.Body, test.status)
			}
		})
	}
}

func TestDownloadRangeAndRedirectOptions(t *testing.T) {
	server := httptest.NewServer(newHandler())
	defer server.Close()
	client := request.New()
	req := model.NewRequest("download")
	req.URL = server.URL + "/download"
	req.Headers = []model.KV{{Key: "Range", Value: "bytes=0-3"}}
	resp, err := client.Send(t.Context(), req, nil)
	if err != nil || resp.StatusCode != 206 || string(resp.Body) != "Reva" {
		t.Fatalf("range = %d, body = %q, err = %v", resp.StatusCode, resp.Body, err)
	}
	if !hasHeader(resp, "Content-Disposition", `attachment; filename="reva-sample.txt"`) {
		t.Fatalf("download missing filename: %+v", resp.Headers)
	}
	req.URL = server.URL + "/redirect"
	req.Headers = nil
	req.Options.FollowRedirects = model.ToggleOff
	resp, err = client.Send(t.Context(), req, nil)
	if err != nil || resp.StatusCode != 302 || !hasHeader(resp, "Location", "/health") {
		t.Fatalf("redirect = %d, headers = %+v, err = %v", resp.StatusCode, resp.Headers, err)
	}
}

func hasHeader(resp model.Response, name, value string) bool {
	for _, header := range resp.Headers {
		if strings.EqualFold(header.Key, name) && header.Value == value {
			return true
		}
	}
	return false
}

func TestCancelledDelay(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	resp := httptest.NewRecorder()
	newHandler().ServeHTTP(resp, httptest.NewRequestWithContext(ctx, "GET", "/delay?ms=10000", nil))
	if resp.Body.Len() != 0 {
		t.Fatalf("cancelled delay wrote %q", resp.Body)
	}
}
