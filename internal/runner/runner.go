// Package runner executes native collections serially without persistence or UI.
package runner

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/runbrick/reva/internal/execution"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
)

// Options configures failure handling. The zero value continues after failures.
type Options struct {
	StopOnFailure bool
}

// Outcome is the disposition of a run or request, independent of HTTP status.
type Outcome string

const (
	Passed    Outcome = "passed"
	Failed    Outcome = "failed"
	Cancelled Outcome = "cancelled"
	Skipped   Outcome = "skipped"
)

// Code is a fixed diagnostic, safe to include in reports.
type Code string

const (
	CodeOK                  Code = "ok"
	CodeAssertion           Code = "assertion_failed"
	CodePreScript           Code = "pre_script_failed"
	CodeTestScript          Code = "test_script_failed"
	CodeAuth                Code = "auth_failed"
	CodeTransport           Code = "transport_failed"
	CodeCancelled           Code = "cancelled"
	CodeUnsupportedMethod   Code = "unsupported_method"
	CodeUnsupportedProtocol Code = "unsupported_protocol"
	CodeSkippedFailure      Code = "skipped_failure"
	CodeSkippedCancellation Code = "skipped_cancellation"
)

// RequestResult retains sensitive diagnostics for in-process consumers only.
// JSON serialization deliberately projects only RequestReport. Do not log this
// value with fmt or serialize Execution directly: both can expose secrets.
type RequestResult struct {
	Ordinal   int               `json:"-"`
	ItemID    string            `json:"-"`
	Outcome   Outcome           `json:"-"`
	Code      Code              `json:"-"`
	Stage     execution.Stage   `json:"-"`
	Duration  time.Duration     `json:"-"`
	Sent      bool              `json:"-"` // underlying sender invoked
	Err       error             `json:"-"` // includes plan construction failures
	Execution *execution.Result `json:"-"`
}

// Result contains outcomes for every request, including unattempted requests.
// Err is the parent context error, never an arbitrary per-request error. It has
// priority over other failures. JSON serialization produces only Report v1.
type Result struct {
	Outcome  Outcome         `json:"-"`
	Err      error           `json:"-"`
	Duration time.Duration   `json:"-"`
	Requests []RequestResult `json:"-"`
}

type job struct {
	id   string
	plan *execution.Plan
	err  error
}

// Run snapshots the entire collection, settings and environment before any send,
// then runs requests in original depth-first order through execution.Plan.
// Environment rows follow collection rows (last enabled value wins). Script
// variables are request-local; no response capture or mutation crosses requests.
// HTTP 4xx/5xx alone are successful. An empty collection succeeds unless ctx is
// cancelled. Callers must not mutate inputs concurrently with snapshot creation.
// sender must be non-nil; its transport state (such as a caller-owned Cookie jar)
// is outside the runner's isolation boundary.
func Run(ctx context.Context, sender service.RequestService, collection model.Collection, settings model.Settings, environment []model.KV, options Options) Result {
	start := time.Now()
	collection = cloneCollection(collection)
	values := append(append([]model.KV(nil), collection.Variables...), environment...)
	var jobs []job
	var walk func([]model.Item)
	walk = func(items []model.Item) {
		for _, item := range items {
			if item.Type == model.ItemRequest && item.Request != nil {
				req := item.Request.Clone()
				// Item identity, rather than a potentially stale embedded request ID,
				// determines the folder path used for inherited authentication.
				req.ID = item.ID
				plan, err := execution.New(execution.Input{Request: req, Collection: collection, Settings: settings, Variables: values})
				jobs = append(jobs, job{id: item.ID, plan: plan, err: err})
			}
			walk(item.Items)
		}
	}
	walk(collection.Items)
	r := Result{Outcome: Passed, Requests: make([]RequestResult, 0, len(jobs))}
	stopped := false
	for i, job := range jobs {
		request := RequestResult{Ordinal: i + 1, ItemID: job.id, Stage: "pending"}
		switch {
		case ctx.Err() != nil:
			request.Outcome, request.Code = Skipped, CodeSkippedCancellation
		case stopped:
			request.Outcome, request.Code = Skipped, CodeSkippedFailure
		default:
			requestStart := time.Now()
			if job.err != nil {
				request.Outcome, request.Code, request.Stage, request.Err = Failed, CodeAuth, "prepare", job.err
			} else {
				guard := &checkedSender{sender: sender}
				detail := job.plan.Run(ctx, guard)
				request.Execution, request.Err = &detail, detail.Err
				request.Stage, request.Sent = detail.Stage, guard.sent
				request.Outcome, request.Code = classify(ctx, detail)
			}
			request.Duration = time.Since(requestStart)
			if request.Outcome == Failed {
				r.Outcome = Failed
				stopped = options.StopOnFailure
			}
		}
		r.Requests = append(r.Requests, request)
	}
	if r.Err = ctx.Err(); r.Err != nil {
		r.Outcome = Cancelled
	}
	r.Duration = time.Since(start)
	return r
}

func classify(ctx context.Context, r execution.Result) (Outcome, Code) {
	switch {
	case ctx.Err() != nil:
		return Cancelled, CodeCancelled
	case r.Pre.Err != nil:
		return Failed, CodePreScript
	case errors.Is(r.Err, errUnsupportedMethod):
		return Failed, CodeUnsupportedMethod
	case errors.Is(r.Err, errUnsupportedProtocol):
		return Failed, CodeUnsupportedProtocol
	case r.Err != nil && r.Stage == execution.StageValidation:
		return Failed, CodeAuth
	case r.Err != nil:
		return Failed, CodeTransport
	case r.Tests.Err != nil:
		return Failed, CodeTestScript
	case r.Failed():
		return Failed, CodeAssertion
	default:
		return Passed, CodeOK
	}
}

var (
	errUnsupportedMethod   = errors.New("unsupported request method")
	errUnsupportedProtocol = errors.New("unsupported request protocol")
)

// checkedSender sees the once-expanded post-pre-script request. It prevents
// unsupported requests from reaching even a permissive injected sender.
type checkedSender struct {
	sender service.RequestService
	sent   bool
}

func (s *checkedSender) Send(ctx context.Context, req model.Request, values []model.KV) (model.Response, error) {
	if err := ctx.Err(); err != nil {
		return model.Response{}, err
	}
	switch req.Method {
	case model.MethodGet, model.MethodPost, model.MethodPut, model.MethodPatch, model.MethodDelete, model.MethodHead, model.MethodOptions:
	default:
		return model.Response{}, errUnsupportedMethod
	}
	// Match the HTTP service's implicit http:// handling for schemeless hosts.
	// Syntax/host validation remains the service's responsibility.
	if scheme, _, explicit := strings.Cut(req.URL, "://"); explicit && !strings.EqualFold(scheme, "http") && !strings.EqualFold(scheme, "https") {
		return model.Response{}, errUnsupportedProtocol
	}
	s.sent = true
	return s.sender.Send(ctx, req, values)
}

func cloneCollection(col model.Collection) model.Collection {
	col.Variables = append([]model.KV(nil), col.Variables...)
	col.Items = cloneItems(col.Items)
	return col
}

func cloneItems(items []model.Item) []model.Item {
	items = append([]model.Item(nil), items...)
	for i := range items {
		items[i].Items = cloneItems(items[i].Items)
		if items[i].Request != nil {
			req := items[i].Request.Clone()
			items[i].Request = &req
		}
	}
	return items
}
