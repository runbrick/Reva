// reva-run executes an explicitly selected native collection without the GUI.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/runbrick/reva/internal/runner"
	"github.com/runbrick/reva/internal/service/request"
)

const (
	exitOK        = 0
	exitFailed    = 1
	exitInput     = 2
	exitCancelled = 130
)

const usage = `Usage: reva-run --collection ID [--data-dir DIR] [--environment ID] [--stop-on-failure]

Runs a native Reva collection serially. No environment is selected by default.
Data directory: --data-dir, then REVA_HOME, then the user config directory/Reva.
Reads existing data only; never saves settings, history, collections or environments.
Native pre-request/test scripts execute; enabled upload files may be read and sent.
Stdout: one compact safe JSON report (version 1) after a run, including cancellation.
Stderr: fixed diagnostics only. Input errors produce no report.
Exit codes: 0 passed/help, 1 run/report-write failure, 2 usage/load error, 130 cancelled.
Use a built binary when consuming exit codes; go run may wrap nonzero exit status.
Flags: --collection ID, --environment ID, --data-dir DIR, --stop-on-failure, -h, --help.
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

// run keeps cancellation and output injectable for real-store/HTTP behavior tests.
// Never print a flag-parser, filesystem, script, transport, or runner raw error:
// every one of those may contain caller-controlled credentials or paths.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("reva-run", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	collectionID := flags.String("collection", "", "")
	environmentID := flags.String("environment", "", "")
	dataDir := flags.String("data-dir", "", "")
	stopOnFailure := flags.Bool("stop-on-failure", false, "")
	help := flags.Bool("help", false, "")
	shortHelp := flags.Bool("h", false, "")
	if err := flags.Parse(args); err != nil {
		return diagnostic(stderr, "invalid_arguments", exitInput)
	}
	if *help || *shortHelp {
		if _, err := io.WriteString(stdout, usage); err != nil {
			return diagnostic(stderr, "output_failed", exitFailed)
		}
		return exitOK
	}
	if flags.NArg() != 0 || !validID(*collectionID) {
		return diagnostic(stderr, "collection_required_or_invalid", exitInput)
	}
	var environmentSet, dataDirSet bool
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "environment":
			environmentSet = true
		case "data-dir":
			dataDirSet = true
		}
	})
	if (environmentSet && !validID(*environmentID)) || (dataDirSet && *dataDir == "") {
		return diagnostic(stderr, "invalid_arguments", exitInput)
	}
	dir, err := readOnlyDir(*dataDir)
	if err != nil {
		return diagnostic(stderr, "data_directory_unavailable", exitInput)
	}
	input, code := loadInput(dir, *collectionID, *environmentID)
	if code != "" {
		return diagnostic(stderr, code, exitInput)
	}
	result := runner.Run(ctx, request.New(), input.collection, input.settings, input.environment,
		runner.Options{StopOnFailure: *stopOnFailure})
	if err := json.NewEncoder(stdout).Encode(result.Report()); err != nil {
		return diagnostic(stderr, "output_failed", exitFailed)
	}
	switch result.Outcome {
	case runner.Cancelled:
		return exitCancelled
	case runner.Passed:
		return exitOK
	default:
		return exitFailed
	}
}

func diagnostic(w io.Writer, code string, exit int) int {
	_, _ = fmt.Fprintln(w, "reva-run:", code)
	return exit
}
