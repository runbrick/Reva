package runner_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/runner"
)

func TestCompactReportParityAndRedaction(t *testing.T) {
	const secret = "private-value-canary"
	col := collection(item(secret, "http://example.invalid/one"), item("failure", "http://example.invalid/two"), item("last", "http://example.invalid/three"))
	col.Items[0].Request.TestScript = `console.log("` + secret + `"); pm.test("` + secret + `", function(){ pm.response.to.have.status(409); });`
	sender := requestFunc(func(_ context.Context, req model.Request, _ []model.KV) (model.Response, error) {
		if req.ID == "failure" {
			return model.Response{}, errors.New(secret)
		}
		return model.Response{StatusCode: 409, Body: []byte(secret)}, nil
	})
	for _, scenario := range []struct {
		name      string
		cancelled bool
		stop      bool
		counts    runner.Counts
	}{
		{name: "continue", counts: runner.Counts{Total: 3, Passed: 2, Failed: 1}},
		{name: "stop", stop: true, counts: runner.Counts{Total: 3, Passed: 1, Failed: 1, Skipped: 1}},
		{name: "pre-cancelled", cancelled: true, counts: runner.Counts{Total: 3, Skipped: 3}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario.cancelled {
				cancel()
			}
			settings := model.DefaultSettings()
			options := runner.Options{StopOnFailure: scenario.stop}
			compact := runner.NewSnapshot(col, settings, nil).RunReport(ctx, sender, options)
			full := runner.Run(ctx, sender, col, settings, nil, options).Report()
			if compact.Counts != scenario.counts || strings.Contains(string(jsonBytes(t, compact)), secret) {
				t.Fatal("compact report counts differ or contain sensitive data")
			}
			compact.DurationNS, full.DurationNS = 0, 0
			for index := range compact.Requests {
				compact.Requests[index].DurationNS, full.Requests[index].DurationNS = 0, 0
			}
			if !reflect.DeepEqual(compact, full) {
				t.Fatalf("compact report %+v differs from detailed projection %+v", compact, full)
			}
		})
	}
}
