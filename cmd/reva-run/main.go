// reva-run executes an explicitly selected native collection without the GUI.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/runbrick/reva/internal/i18n"
	"github.com/runbrick/reva/internal/runner"
	"github.com/runbrick/reva/internal/service/request"
)

const (
	exitOK        = 0
	exitFailed    = 1
	exitInput     = 2
	exitCancelled = 130
)

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
	folderID := flags.String("folder", "", "")
	environmentID := flags.String("environment", "", "")
	dataDir := flags.String("data-dir", "", "")
	dataFile := flags.String("data-file", "", "")
	reportFormat := flags.String("report-format", "json", "")
	stopOnFailure := flags.Bool("stop-on-failure", false, "")
	cookieSession := flags.Bool("cookie-session", false, "")
	help := flags.Bool("help", false, "")
	shortHelp := flags.Bool("h", false, "")
	if err := flags.Parse(args); err != nil {
		return diagnostic(stderr, "invalid_arguments", exitInput)
	}
	if *help || *shortHelp {
		if _, err := io.WriteString(stdout, i18n.RunnerFor(i18n.Zh).Help+"\n"+i18n.RunnerFor(i18n.En).Help); err != nil {
			return diagnostic(stderr, "output_failed", exitFailed)
		}
		return exitOK
	}
	if flags.NArg() != 0 || !validID(*collectionID) {
		return diagnostic(stderr, "collection_required_or_invalid", exitInput)
	}
	var invalidSelection bool
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "environment":
			invalidSelection = invalidSelection || !validID(*environmentID)
		case "folder":
			invalidSelection = invalidSelection || !validID(*folderID)
		case "data-dir":
			invalidSelection = invalidSelection || *dataDir == ""
		case "data-file":
			invalidSelection = invalidSelection || *dataFile == ""
		}
	})
	if invalidSelection || (*reportFormat != "json" && *reportFormat != "junit") {
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
	selected := runner.NewSnapshot(input.collection, input.settings, input.environment)
	if *folderID != "" {
		selected, err = selected.WithFolder(*folderID)
		if err != nil {
			return diagnostic(stderr, "folder_invalid", exitInput)
		}
	}
	sender := request.New()
	options := runner.Options{StopOnFailure: *stopOnFailure, UseCookieSession: *cookieSession}
	var report runner.Report
	if *dataFile == "" {
		report = selected.RunReport(ctx, sender, options)
	} else {
		data, err := runner.ReadDataFile(*dataFile)
		if err != nil {
			return diagnostic(stderr, "data_file_invalid_or_unreadable", exitInput)
		}
		report, err = selected.RunDataReport(ctx, sender, options, data)
		if errors.Is(err, runner.ErrIterationLimit) {
			return diagnostic(stderr, "iteration_limit_exceeded", exitInput)
		}
		if err != nil {
			return diagnostic(stderr, "data_file_invalid_or_unreadable", exitInput)
		}
	}
	if *reportFormat == "junit" {
		err = runner.WriteJUnit(stdout, report)
	} else {
		err = json.NewEncoder(stdout).Encode(report)
	}
	if err != nil {
		return diagnostic(stderr, "output_failed", exitFailed)
	}
	switch report.Outcome {
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
