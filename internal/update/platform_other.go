//go:build !windows && !darwin

package update

import (
	"errors"
	"os/exec"
	"time"
)

const helperName = "reva-update-helper"

var errUnsupported = errors.New("in-app installation is unavailable on this platform")

func installation(string) (string, string)   { return "", "" }
func detach(*exec.Cmd)                       {}
func installSetup(string, string) error      { return errUnsupported }
func launch(string, string, ...string) error { return errUnsupported }

func processWaiter(int, time.Duration) (func() error, func(), error) {
	return nil, nil, errUnsupported
}
