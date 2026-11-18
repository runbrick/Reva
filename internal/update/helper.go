package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	ApplyCommand   = "--reva-apply-update"
	RestartCommand = "--reva-after-update"
	stagePrefix    = ".reva-update-"
	previousName   = "previous"
	readyByte      = byte(1)
)

func payloadName(kind string) string {
	switch kind {
	case setup:
		return "setup.exe"
	case portable:
		return "reva.exe"
	default:
		return "Reva.app"
	}
}

func handOff(stage, target, kind string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	source, err := os.Open(executable)
	if err != nil {
		return err
	}
	defer source.Close()
	helper := filepath.Join(stage, helperName)
	file, err := os.OpenFile(helper, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, source)
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return err
	}
	cmd := exec.Command(helper, ApplyCommand, strconv.Itoa(os.Getpid()), target, kind)
	cmd.Dir = filepath.Dir(target)
	detach(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	ready := make(chan bool, 1)
	go func() {
		var signal [1]byte
		_, err := io.ReadFull(stdout, signal[:])
		ready <- err == nil && signal[0] == readyByte
	}()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case ok := <-ready:
		if ok {
			_ = stdout.Close()
			_ = cmd.Process.Release()
			return nil
		}
	case <-timer.C:
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	return errors.New("update helper did not start")
}

func validStage(name string) bool {
	serial, ok := strings.CutPrefix(name, stagePrefix)
	return ok && serial != "" && len(serial) <= 10 && strings.Trim(serial, "0123456789") == ""
}

func RunHelper(args []string) error {
	if len(args) != 3 || !filepath.IsAbs(args[1]) {
		return errors.New("invalid update job")
	}
	parent, err := strconv.Atoi(args[0])
	if err != nil || parent <= 0 || parent == os.Getpid() {
		return errors.New("invalid update parent process")
	}
	target, kind := filepath.Clean(args[1]), args[2]
	installed, actualKind := installation(executableFor(target, kind))
	if installed != target || actualKind != kind {
		return errors.New("invalid update target")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	stage := filepath.Dir(self)
	stageName := filepath.Base(stage)
	if filepath.Base(self) != helperName || !validStage(stageName) || filepath.Dir(stage) != filepath.Dir(target) {
		return errors.New("invalid update staging directory")
	}
	payload := filepath.Join(stage, payloadName(kind))
	if info, err := os.Lstat(payload); err != nil || (kind == bundle && !info.IsDir()) || (kind != bundle && !info.Mode().IsRegular()) {
		return errors.New("invalid staged update")
	}
	wait, closeProcess, err := processWaiter(parent, 90*time.Second)
	if err != nil {
		return err
	}
	defer closeProcess()
	if _, err := os.Stdout.Write([]byte{readyByte}); err != nil {
		return err
	}
	if err := wait(); err != nil {
		return err
	}
	previous := filepath.Join(stage, previousName)
	start := func(failed bool) error {
		status := "ok"
		if failed {
			status = "failed"
		}
		return launch(target, kind, RestartCommand, strconv.Itoa(os.Getpid()), stageName, status)
	}
	apply := func() error {
		if kind == setup {
			return installSetup(payload, filepath.Dir(target))
		}
		return os.Rename(payload, target)
	}
	err = replace(target, previous, apply, func() error { return start(false) })
	if err != nil {
		if _, statErr := os.Stat(previous); errors.Is(statErr, os.ErrNotExist) {
			return errors.Join(err, start(true))
		}
		return err
	}
	return nil
}

func replace(target, previous string, apply, start func() error) error {
	if err := os.Rename(target, previous); err != nil {
		return err
	}
	err := apply()
	if err == nil {
		err = start()
	}
	if err != nil {
		if removeErr := os.RemoveAll(target); removeErr != nil {
			return errors.Join(err, removeErr)
		}
		return errors.Join(err, os.Rename(previous, target))
	}
	return nil
}

func FinishRestart(args []string) bool {
	if len(args) != 3 || !validStage(args[1]) || (args[2] != "ok" && args[2] != "failed") {
		return false
	}
	failed := args[2] == "failed"
	helperPID, err := strconv.Atoi(args[0])
	if err != nil || helperPID <= 0 || helperPID == os.Getpid() {
		return failed
	}
	exe, err := os.Executable()
	if err != nil {
		return failed
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return failed
	}
	target, kind := installation(exe)
	if kind == "" {
		return failed
	}
	wait, closeProcess, err := processWaiter(helperPID, 3*time.Second)
	if err != nil {
		return failed
	}
	defer closeProcess()
	if wait() != nil {
		return failed
	}
	stage := filepath.Join(filepath.Dir(target), args[1])
	if info, err := os.Lstat(stage); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return failed
	}
	if info, err := os.Stat(filepath.Join(stage, helperName)); err != nil || !info.Mode().IsRegular() {
		return failed
	}
	if failed {
		for _, name := range []string{payloadName(kind), helperName} {
			_ = os.RemoveAll(filepath.Join(stage, name))
		}
		_ = os.Remove(stage)
	} else if err := os.RemoveAll(stage); err != nil {
		fmt.Fprintln(os.Stderr, "reva update cleanup:", err)
	}
	return failed
}

func executableFor(target, kind string) string {
	if kind == bundle {
		return filepath.Join(target, "Contents", "MacOS", "reva")
	}
	return target
}
