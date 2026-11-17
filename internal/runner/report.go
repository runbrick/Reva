package runner

import (
	"encoding/json"
	"time"

	"github.com/runbrick/reva/internal/execution"
)

// ReportVersion versions only this report DTO, independently of model schemas.
const ReportVersion = 1

// Counts includes every request. Cancelled counts an interrupted attempt;
// unattempted requests count as Skipped, including pre-cancelled runs.
type Counts struct {
	Total     int `json:"total"`
	Passed    int `json:"passed"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
	Skipped   int `json:"skipped"`
}

// Assertions contains counts only; assertion names and errors are untrusted.
type Assertions struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

// Report is the v1 export allowlist. It contains no caller-supplied strings or
// identifiers. Ordinals identify positions within this run, not stored entities.
// Status codes, elapsed times, counts and outcomes are intentional disclosures.
// Serialize Result or Result.Report(); never add raw diagnostic fields to this
// DTO. Modifying a projected DTO afterwards is outside this safety contract.
type Report struct {
	Version    int             `json:"version"`
	Outcome    Outcome         `json:"outcome"`
	DurationNS int64           `json:"durationNs"`
	Counts     Counts          `json:"counts"`
	Assertions Assertions      `json:"assertions"`
	Requests   []RequestReport `json:"requests"`
}

// RequestReport deliberately omits IDs, names, URLs, methods, raw request and
// response fields, variables, scripts, logs, and arbitrary error strings.
type RequestReport struct {
	Ordinal    int             `json:"ordinal"`
	Outcome    Outcome         `json:"outcome"`
	Code       Code            `json:"code"`
	Stage      execution.Stage `json:"stage"`
	Sent       bool            `json:"sent"`
	StatusCode int             `json:"statusCode,omitempty"`
	DurationNS int64           `json:"durationNs"`
	Assertions Assertions      `json:"assertions"`
}

// Report projects only allowlisted metadata, including when callers have
// modified exported diagnostic fields. Unknown enum values become fixed codes.
func (r Result) Report() Report {
	report := Report{Version: ReportVersion, Outcome: safeOutcome(r.Outcome), DurationNS: elapsed(r.Duration), Requests: make([]RequestReport, len(r.Requests))}
	report.Counts.Total = len(r.Requests)
	for i, request := range r.Requests {
		entry := request.report(i + 1)
		report.Requests[i] = entry
		switch entry.Outcome {
		case Passed:
			report.Counts.Passed++
		case Failed:
			report.Counts.Failed++
		case Cancelled:
			report.Counts.Cancelled++
		case Skipped:
			report.Counts.Skipped++
		}
		report.Assertions.Passed += entry.Assertions.Passed
		report.Assertions.Failed += entry.Assertions.Failed
	}
	return report
}

// MarshalJSON makes ordinary json.Marshal(Result) safe by default.
func (r Result) MarshalJSON() ([]byte, error) { return json.Marshal(r.Report()) }

// MarshalJSON also protects individual request diagnostics from accidental JSON
// exposure. Only its numeric position is retained from the original identity.
func (r RequestResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.report(max(0, r.Ordinal)))
}

func (r RequestResult) report(ordinal int) RequestReport {
	entry := RequestReport{Ordinal: ordinal, Outcome: safeOutcome(r.Outcome), Code: safeCode(r.Code), Stage: safeStage(r.Stage), Sent: r.Sent, DurationNS: elapsed(r.Duration)}
	if r.Execution != nil {
		if response := r.Execution.Response; response != nil && response.StatusCode >= 100 && response.StatusCode <= 599 {
			entry.StatusCode = response.StatusCode
		}
		for _, assertion := range r.Execution.Tests.Assertions {
			if assertion.Passed {
				entry.Assertions.Passed++
			} else {
				entry.Assertions.Failed++
			}
		}
	}
	return entry
}

func elapsed(duration time.Duration) int64 { return int64(max(0, duration)) }

func safeOutcome(outcome Outcome) Outcome {
	switch outcome {
	case Passed, Failed, Cancelled, Skipped:
		return outcome
	default:
		return Failed
	}
}

func safeCode(code Code) Code {
	switch code {
	case CodeOK, CodeAssertion, CodePreScript, CodeTestScript, CodeAuth, CodeTransport, CodeCancelled, CodeUnsupportedMethod, CodeUnsupportedProtocol, CodeSkippedFailure, CodeSkippedCancellation:
		return code
	default:
		return CodeTransport
	}
}

func safeStage(stage execution.Stage) execution.Stage {
	switch stage {
	case "pending", "prepare", execution.StagePre, execution.StageValidation, execution.StageTransport, execution.StageTests, execution.StageComplete:
		return stage
	default:
		return "pending"
	}
}
