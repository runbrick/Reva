// Package script runs isolated, time-limited pre-request and response JavaScript.
package script

import (
	_ "embed"
	"errors"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/vars"
)

// DefaultTimeout bounds scripts whose caller does not supply a positive timeout.
const DefaultTimeout = 2 * time.Second

// ErrTimeout is returned when execution exceeds its time budget.
var ErrTimeout = errors.New("script timed out")

// Result records one pm.test callback, including assertion or JavaScript errors.
type Result struct {
	Name   string
	Passed bool
	Error  string
}

//go:embed api.js
var apiSource string

var apiProgram = func() *otto.Script {
	program, err := otto.New().Compile("reva-api.js", apiSource)
	if err != nil {
		panic(err)
	}
	return program
}()

// A private panic value bypasses JavaScript catch blocks and is recovered only
// at the runner boundary, as required by Otto's Interrupt API.
type timeoutSignal struct{}

type runner struct {
	vm      *otto.Otto
	vars    []model.KV
	logs    []string
	results []Result
}

// RunPre executes src and writes request mutations back immediately. Variables
// are copied; changes apply to this send without modifying the caller's scopes.
func RunPre(src string, req *model.Request, values []model.KV, timeout time.Duration) (newVars []model.KV, logs []string, err error) {
	if req == nil {
		return append([]model.KV(nil), values...), nil, errors.New("script request is nil")
	}
	r := newRunner(req, nil, values)
	err = r.run(src, timeout)
	return r.vars, r.logs, err
}

// RunTests executes src against a request snapshot and response. Failed tests
// are results; syntax errors, uncaught exceptions and timeouts also return err.
func RunTests(src string, req model.Request, resp model.Response, values []model.KV, timeout time.Duration) (results []Result, logs []string, err error) {
	req = req.Clone()
	r := newRunner(&req, &resp, values)
	err = r.run(src, timeout)
	return r.results, r.logs, err
}

func newRunner(req *model.Request, resp *model.Response, values []model.KV) *runner {
	r := &runner{vm: otto.New(), vars: append([]model.KV(nil), values...)}
	host := map[string]any{
		"url":       func() string { return req.URL },
		"setURL":    func(v string) { req.URL = v },
		"method":    func() string { return string(req.Method) },
		"setMethod": func(v string) { req.Method = model.Method(v) },
		"addHeader": func(key, value string) {
			req.Headers = append(req.Headers, model.KV{Key: key, Value: value})
		},
		"removeHeader": func(key string) {
			out := req.Headers[:0]
			for _, h := range req.Headers {
				if !strings.EqualFold(h.Key, key) {
					out = append(out, h)
				}
			}
			req.Headers = out
		},
		"upsertHeader": func(key, value string) {
			out := req.Headers[:0]
			found := false
			for _, h := range req.Headers {
				if strings.EqualFold(h.Key, key) {
					if found {
						continue
					}
					h = model.KV{Key: key, Value: value}
					found = true
				}
				out = append(out, h)
			}
			if !found {
				out = append(out, model.KV{Key: key, Value: value})
			}
			req.Headers = out
		},
		"getVar": func(key string) otto.Value {
			if value, ok := vars.Lookup(r.vars, key); ok {
				return stringValue(value)
			}
			return otto.UndefinedValue()
		},
		"setVar": func(key, value string) {
			for i := len(r.vars) - 1; i >= 0; i-- {
				if r.vars[i].Key == key {
					r.vars[i].Value, r.vars[i].Disabled = value, false
					return
				}
			}
			r.vars = append(r.vars, model.KV{Key: key, Value: value})
		},
		"log": func(line string) { r.logs = append(r.logs, line) },
		"result": func(name string, passed bool, message string) {
			r.results = append(r.results, Result{Name: name, Passed: passed, Error: message})
		},
	}
	if resp != nil {
		host["response"] = map[string]any{
			"code": resp.StatusCode, "status": resp.Status,
			"responseTime": float64(resp.Duration) / float64(time.Millisecond),
		}
		host["text"] = func() string { return string(resp.Body) }
		host["header"] = func(name string) otto.Value {
			for _, h := range resp.Headers {
				if !h.Disabled && strings.EqualFold(h.Key, name) {
					return stringValue(h.Value)
				}
			}
			return otto.UndefinedValue()
		}
	}
	// These properties are ordinary Go functions and data; Set cannot invoke JS.
	if err := r.vm.Set("__revaHost", host); err != nil {
		panic(err)
	}
	return r
}

func stringValue(s string) otto.Value {
	// Converting a Go string to a JS primitive cannot fail.
	value, err := otto.ToValue(s)
	if err != nil {
		panic(err)
	}
	return value
}

func (r *runner) run(src string, timeout time.Duration) (err error) {
	expired := false
	defer func() {
		if caught := recover(); caught != nil {
			if _, ok := caught.(timeoutSignal); ok {
				err = ErrTimeout
			} else {
				panic(caught)
			}
		}
		// Otto converts foreign panics inside JS try/catch into TypeErrors. The
		// sticky interrupt below prevents JS from resuming, and this preserves
		// the timeout error when Otto returns that converted error instead.
		if expired {
			err = ErrTimeout
		}
	}()
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	// Only this buffered channel crosses the goroutine boundary. A fired timer
	// never blocks even if execution just ended; no timer runs while UI is idle.
	interrupt := make(chan func(), 1)
	r.vm.Interrupt = interrupt
	var halt func()
	halt = func() {
		expired = true
		// Re-arm before panicking: even a JS catch/finally block is interrupted
		// on its first statement. The channel has just been drained by Otto.
		interrupt <- halt
		panic(timeoutSignal{})
	}
	timer := time.AfterFunc(timeout, func() {
		interrupt <- halt
	})
	defer timer.Stop()
	if _, err := r.vm.Run(apiProgram); err != nil {
		return err
	}
	_, err = r.vm.Run(src)
	return err
}
