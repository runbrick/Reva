package update

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const helperName = "reva-update-helper"

func installation(executable string) (string, string) {
	if strings.Contains(executable, "/AppTranslocation/") || strings.HasPrefix(executable, "/Volumes/") {
		return "", ""
	}
	dir := filepath.Dir(executable)
	root := filepath.Dir(filepath.Dir(dir))
	if !strings.HasSuffix(root, ".app") || executableFor(root, bundle) != executable {
		return "", ""
	}
	return root, bundle
}

func detach(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

func processWaiter(pid int, timeout time.Duration) (func() error, func(), error) {
	if pid <= 0 || int(int32(pid)) != pid {
		return nil, nil, errors.New("invalid macOS process ID")
	}
	return func() error {
		deadline := time.Now().Add(timeout)
		for {
			err := syscall.Kill(pid, 0)
			if errors.Is(err, syscall.ESRCH) {
				return nil
			}
			if err != nil {
				return err
			}
			if time.Now().After(deadline) {
				return errors.New("timed out waiting for Reva to exit")
			}
			time.Sleep(50 * time.Millisecond)
		}
	}, func() {}, nil
}

func installSetup(string, string) error { return errors.New("Windows installer on macOS") }

func launch(target, _ string, args ...string) error {
	return exec.Command("/usr/bin/open", append([]string{"-n", target, "--args"}, args...)...).Run()
}
