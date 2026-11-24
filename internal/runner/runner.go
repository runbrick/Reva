// Package runner executes native collections serially without persistence or UI.
package runner

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/runbrick/reva/internal/execution"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/service/request"
)

// Options configures execution. The zero value continues after failures without a cookie jar.
type Options struct {
	StopOnFailure    bool
	UseCookieSession bool
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
	Iteration int               `json:"-"`
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

// Snapshot owns the saved inputs of a run. Create it before dispatching work
// when the caller needs to fix the inputs on its own thread.
type Snapshot struct {
	collection model.Collection
	requests   []*model.Item
	settings   model.Settings
	values     []model.KV
}

// NewSnapshot copies all execution inputs, including request and variable rows.
func NewSnapshot(collection model.Collection, settings model.Settings, environment []model.KV) Snapshot {
	collection = cloneCollection(collection)
	return Snapshot{
		collection: collection,
		requests:   requestItems(collection.Items),
		settings:   settings,
		values:     append(append([]model.KV(nil), collection.Variables...), environment...),
	}
}

// WithFolder restricts execution to a saved folder ID. An empty ID selects the
// whole collection. The full tree remains available for inherited auth.
func (s Snapshot) WithFolder(folderID string) (Snapshot, error) {
	if folderID == "" {
		s.requests = requestItems(s.collection.Items)
		return s, nil
	}
	folder := model.FindItem(s.collection.Items, folderID)
	if folder == nil || folder.Type != model.ItemFolder {
		return Snapshot{}, ErrInvalidFolder
	}
	s.requests = requestItems(folder.Items)
	return s, nil
}

var ErrInvalidFolder = errors.New("invalid run folder")

func requestItems(items []model.Item) []*model.Item {
	var requests []*model.Item
	var walk func([]model.Item)
	walk = func(nodes []model.Item) {
		for index := range nodes {
			item := &nodes[index]
			if item.Type == model.ItemRequest && item.Request != nil {
				requests = append(requests, item)
			}
			walk(item.Items)
		}
	}
	walk(items)
	return requests
}

// Run snapshots the entire collection, settings and environment before any send,
// then runs requests in original depth-first order through execution.Plan.
// Environment rows follow collection rows (last enabled value wins). Script
// edits are request-local. Explicit response captures from successful requests
// override saved values within this run.
// HTTP 4xx/5xx alone are successful. An empty collection succeeds unless ctx is
// cancelled. Callers must not mutate inputs concurrently with snapshot creation.
// sender must be non-nil. An opted-in runner cookie session belongs to a single
// iteration. Transport state supplied by the caller remains caller-owned.
func Run(ctx context.Context, sender service.RequestService, collection model.Collection, settings model.Settings, environment []model.KV, options Options) Result {
	start := time.Now()
	snapshot := NewSnapshot(collection, settings, environment)
	details := make([]RequestResult, 0)
	outcome := snapshot.run(ctx, sender, options, IterationData{}, func(request RequestResult) {
		details = append(details, request)
	})
	err := ctx.Err()
	if err != nil {
		outcome = Cancelled
	}
	return Result{Outcome: outcome, Err: err, Duration: time.Since(start), Requests: details}
}

func (s Snapshot) run(ctx context.Context, sender service.RequestService, options Options, data IterationData, record func(RequestResult)) Outcome {
	outcome := Passed
	ordinal := 0
	stopped := false
	for index := range max(1, len(data.rows)) {
		iteration, base, iterationCtx := 0, s.values, ctx
		if len(data.rows) != 0 {
			iteration = index + 1
			if !stopped && ctx.Err() == nil && len(s.requests) != 0 {
				base = append(slices.Clone(base), data.rows[index]...)
			}
		}
		if options.UseCookieSession && !stopped && ctx.Err() == nil && len(s.requests) != 0 {
			iterationCtx = request.WithCookieSession(ctx, request.NewCookieSession())
		}
		variables := base
		var captured []model.KV
		for _, item := range s.requests {
			ordinal++
			result := RequestResult{Ordinal: ordinal, Iteration: iteration, ItemID: item.ID, Stage: "pending"}
			switch {
			case ctx.Err() != nil:
				result.Outcome, result.Code = Skipped, CodeSkippedCancellation
			case stopped:
				result.Outcome, result.Code = Skipped, CodeSkippedFailure
			default:
				result = s.execute(iterationCtx, sender, item, result, variables)
				if changes := result.Execution; changes != nil && len(changes.Captures) != 0 {
					if result.Outcome == Passed {
						next, ok := mergeCaptures(captured, changes.Captures)
						if ok {
							captured = next
							variables = append(slices.Clone(base), captured...)
						} else {
							changes.Tests.Err = errCaptureLimit
							changes.Stage = execution.StageTests
							result.Outcome, result.Code, result.Stage = Failed, CodeTestScript, execution.StageTests
						}
					}
					changes.Captures = nil
				}
				if result.Outcome == Failed {
					outcome = Failed
					stopped = options.StopOnFailure
				}
			}
			record(result)
		}
	}
	return outcome
}

func mergeCaptures(existing, incoming []model.KV) ([]model.KV, bool) {
	const maxRunCaptures = 128
	next := slices.Clone(existing)
	for _, capture := range incoming {
		index := slices.IndexFunc(next, func(row model.KV) bool { return row.Key == capture.Key })
		if index >= 0 {
			next[index] = capture
		} else {
			if len(next) == maxRunCaptures {
				return nil, false
			}
			next = append(next, capture)
		}
	}
	return next, true
}

func (s Snapshot) execute(ctx context.Context, sender service.RequestService, item *model.Item, request RequestResult, variables []model.KV) RequestResult {
	req := *item.Request
	req.ID = item.ID
	if ctx.Err() != nil {
		request.Outcome, request.Code = Skipped, CodeSkippedCancellation
		return request
	}
	if req.Method == model.MethodGRPC {
		request.Outcome, request.Code, request.Stage, request.Err = Failed, CodeUnsupportedProtocol, execution.StageValidation, errUnsupportedProtocol
		return request
	}
	plan, err := execution.New(execution.Input{Request: req, Collection: s.collection, Settings: s.settings, Variables: variables})
	if ctx.Err() != nil {
		request.Outcome, request.Code = Skipped, CodeSkippedCancellation
		return request
	}
	start := time.Now()
	if err != nil {
		request.Outcome, request.Code, request.Stage, request.Err = Failed, CodeAuth, "prepare", err
	} else {
		guard := &checkedSender{sender: sender}
		detail := plan.Run(ctx, guard)
		request.Execution, request.Err = &detail, detail.Err
		request.Stage, request.Sent = detail.Stage, guard.sent
		request.Outcome, request.Code = classify(ctx, detail)
	}
	request.Duration = time.Since(start)
	return request
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
	errCaptureLimit        = errors.New("response capture limit reached")
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
	case model.MethodGRPC:
		return model.Response{}, errUnsupportedProtocol
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
