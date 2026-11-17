// Package execution runs one request without GUI or persistence dependencies.
package execution

import (
	"context"
	"strings"

	"github.com/runbrick/reva/internal/auth"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/script"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/vars"
)

// Input is a single send's scope. Variables are already ordered by precedence
// (collection followed by environment, for example). Collection is needed only
// for inherited auth; Request.ID must identify an actual item in that tree.
type Input struct {
	Request    model.Request
	Collection model.Collection
	Variables  []model.KV
	Settings   model.Settings
}

// Plan owns an immutable snapshot with resolved, unexpanded authentication and
// transport options. New must be called before the caller changes its input.
// A plan may be run more than once, including concurrently.
type Plan struct {
	request model.Request
	values  []model.KV
	history model.Request
}

// New clones the input, resolves inherited authentication by Request.ID, and
// resolves transport options. It never runs scripts, expands values, or sends.
// Missing inheritance context is the only construction failure.
func New(in Input) (*Plan, error) {
	req := in.Request.Clone()
	cfg, _, err := auth.Resolve(in.Collection, req.ID, req.Auth)
	if err != nil {
		return nil, err
	}
	history := req.Clone()
	history.Auth = cfg
	req.Auth = cfg
	req.Options = req.Options.Resolve(in.Settings)
	return &Plan{request: req, values: append([]model.KV(nil), in.Variables...), history: history}, nil
}

// Request returns a copy before scripting and expansion, with resolved options.
func (p *Plan) Request() model.Request { return p.request.Clone() }

// Variables returns a copy of the original variable snapshot.
func (p *Plan) Variables() []model.KV { return append([]model.KV(nil), p.values...) }

// ValidateAuth supports immediate GUI feedback for script-free sends. Run
// always validates again after pre-script execution with its runtime variables.
func (p *Plan) ValidateAuth() error { return auth.Validate(vars.Apply(p.request, p.values).Auth) }

// Stage is the last stage entered, including on failure or cancellation.
type Stage string

const (
	StagePre        Stage = "pre"
	StageValidation Stage = "validation"
	StageTransport  Stage = "transport"
	StageTests      Stage = "tests"
	StageComplete   Stage = "complete"
)

// ScriptResult keeps script exceptions/timeouts separate from assertions and
// operational errors. Logs and assertions before an exception are retained.
type ScriptResult struct {
	Logs       []string
	Assertions []script.Result
	Err        error
}

// Result contains runtime data, which can include secrets and must not be
// serialized into reports or history indiscriminately. HistoryRequest is the
// original unexpanded draft with resolved unexpanded auth, without script edits.
// Err is validation, cancellation, or transport failure; script errors live in
// Pre/Tests. HTTP error statuses remain responses with Err == nil.
type Result struct {
	Stage            Stage
	Response         *model.Response
	Err              error
	Pre, Tests       ScriptResult
	TransportStarted bool
	Prepared         model.Request
	Variables        []model.KV
	HistoryRequest   model.Request
}

// Failed reports operational/script failures and failed assertions. HTTP status
// alone does not fail execution; use a response assertion to require a status.
func (r Result) Failed() bool {
	if r.Err != nil || r.Pre.Err != nil || r.Tests.Err != nil {
		return true
	}
	for _, assertion := range r.Tests.Assertions {
		if !assertion.Passed {
			return true
		}
	}
	return false
}

// Run executes pre-script, one variable expansion, auth validation, transport,
// then response tests. Pre-script failure aborts transport. Cancellation is
// checked between stages; running scripts remain bounded by DefaultTimeout.
// The service receives the expanded request and nil variables, preventing a
// second substitution of placeholders that occur inside variable values.
// Neither the engine nor scripts persist changes to inputs or variables.
func (p *Plan) Run(ctx context.Context, svc service.RequestService) Result {
	r := Result{Stage: StagePre, Prepared: p.Request(), Variables: p.Variables(), HistoryRequest: p.history.Clone()}
	if r.Err = ctx.Err(); r.Err != nil {
		return r
	}
	if strings.TrimSpace(r.Prepared.PreScript) != "" {
		r.Variables, r.Pre.Logs, r.Pre.Err = script.RunPre(r.Prepared.PreScript, &r.Prepared, r.Variables, 0)
	}
	if r.Err = ctx.Err(); r.Err != nil || r.Pre.Err != nil {
		return r
	}
	r.Stage = StageValidation
	expanded := vars.Apply(r.Prepared, r.Variables)
	if r.Err = auth.Validate(expanded.Auth); r.Err != nil {
		return r
	}
	if r.Err = ctx.Err(); r.Err != nil {
		return r
	}
	r.Stage, r.TransportStarted = StageTransport, true
	resp, err := svc.Send(ctx, expanded, nil)
	r.Err = err
	if ctx.Err() != nil {
		r.Err = ctx.Err()
	}
	if r.Err != nil {
		return r
	}
	r.Response = &resp
	r.Stage = StageTests
	if strings.TrimSpace(r.Prepared.TestScript) != "" {
		r.Tests.Assertions, r.Tests.Logs, r.Tests.Err = script.RunTests(r.Prepared.TestScript, expanded, resp, r.Variables, 0)
	}
	if r.Err = ctx.Err(); r.Err != nil || r.Tests.Err != nil {
		return r
	}
	r.Stage = StageComplete
	return r
}
