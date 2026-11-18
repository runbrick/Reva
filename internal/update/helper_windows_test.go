package update

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const fixtureCommand = "--test-start-update"
const fixtureResult = "REVA_UPDATE_TEST_RESULT"

func TestMain(tests *testing.M) {
	if len(os.Args) < 2 || os.Getenv(fixtureResult) == "" {
		os.Exit(tests.Run())
	}
	var err error
	switch os.Args[1] {
	case fixtureCommand:
		err = handOff(os.Args[2], os.Args[0], portable)
	case ApplyCommand:
		err = RunHelper(os.Args[2:])
	case RestartCommand:
		failed := FinishRestart(os.Args[2:])
		err = os.WriteFile(os.Getenv(fixtureResult), []byte(fmt.Sprint(failed)), 0o600)
	default:
		os.Exit(tests.Run())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		_ = os.WriteFile(os.Getenv(fixtureResult)+"-error", []byte(err.Error()), 0o600)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestWindowsPortableHelperReplacesAndRelaunches(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(fmt.Sprintf("rollback=%t", broken), func(t *testing.T) {
			dir := t.TempDir()
			result := filepath.Join(dir, "restarted")
			t.Setenv(fixtureResult, result)
			stage, err := os.MkdirTemp(dir, stagePrefix)
			if err != nil {
				t.Fatal(err)
			}
			current, err := os.ReadFile(os.Args[0])
			if err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "reva.exe")
			if err := os.WriteFile(target, current, 0o700); err != nil {
				t.Fatal(err)
			}
			candidate := append(current, []byte("new version")...)
			if broken {
				candidate = []byte("not an executable")
			}
			if err := os.WriteFile(filepath.Join(stage, payloadName(portable)), candidate, 0o700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			defer cancel()
			if output, err := exec.CommandContext(ctx, target, fixtureCommand, stage).CombinedOutput(); err != nil {
				t.Fatalf("could not stage fixture: %v: %s", err, output)
			}
			for {
				data, err := os.ReadFile(result)
				if err == nil && len(data) > 0 {
					if string(data) != fmt.Sprint(broken) {
						t.Fatalf("unexpected restart result: %s", data)
					}
					break
				}
				if err != nil && !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				select {
				case <-ctx.Done():
					diagnostic, _ := os.ReadFile(result + "-error")
					t.Fatalf("helper did not relaunch Reva: %s", diagnostic)
				case <-time.After(50 * time.Millisecond):
				}
			}
			want := candidate
			if broken {
				want = current
			}
			if got, err := os.ReadFile(target); err != nil || !bytes.Equal(got, want) {
				t.Fatalf("unexpected executable after update: %v", err)
			}
			if _, err := os.Stat(stage); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("staging directory was not removed: %v", err)
			}
		})
	}
}
