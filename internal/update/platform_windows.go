package update

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	helperName         = "reva-update-helper.exe"
	detachedProcess    = 0x00000008
	synchronizeProcess = 0x00100000
	invalidProcessID   = syscall.Errno(87)
)

func installation(executable string) (string, string) {
	if !strings.EqualFold(filepath.Base(executable), "reva.exe") {
		return "", ""
	}
	for dir := filepath.Dir(executable); ; dir = filepath.Dir(dir) {
		if strings.HasPrefix(filepath.Base(dir), "go-build") {
			return "", ""
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	if installedAt(syscall.HKEY_CURRENT_USER, executable) {
		return executable, setup
	}
	if installedAt(syscall.HKEY_LOCAL_MACHINE, executable) {
		return "", ""
	}
	return executable, portable
}

func installedAt(hive syscall.Handle, executable string) bool {
	key, _ := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Uninstall\com.runbrick.reva`)
	var handle syscall.Handle
	if syscall.RegOpenKeyEx(hive, key, 0, syscall.KEY_QUERY_VALUE|syscall.KEY_WOW64_64KEY, &handle) != nil {
		return false
	}
	defer syscall.RegCloseKey(handle)
	name, _ := syscall.UTF16PtrFromString("InstallLocation")
	var valueType uint32
	var data [32768]uint16
	size := uint32(len(data) * 2)
	if syscall.RegQueryValueEx(handle, name, nil, &valueType, (*byte)(unsafe.Pointer(&data[0])), &size) != nil || valueType != syscall.REG_SZ {
		return false
	}
	dir, err := filepath.EvalSymlinks(syscall.UTF16ToString(data[:size/2]))
	return err == nil && strings.EqualFold(filepath.Dir(executable), dir)
}

func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess}
}

func processWaiter(pid int, timeout time.Duration) (func() error, func(), error) {
	if uint64(pid) > uint64(^uint32(0)) {
		return nil, nil, errors.New("invalid Windows process ID")
	}
	handle, err := syscall.OpenProcess(synchronizeProcess, false, uint32(pid))
	if errors.Is(err, invalidProcessID) {
		return func() error { return nil }, func() {}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return func() error {
		result, err := syscall.WaitForSingleObject(handle, uint32(timeout.Milliseconds()))
		if err != nil {
			return err
		}
		if result != syscall.WAIT_OBJECT_0 {
			return errors.New("timed out waiting for Reva to exit")
		}
		return nil
	}, func() { _ = syscall.CloseHandle(handle) }, nil
}

func installSetup(path, dir string) error {
	cmd := exec.Command(path)
	cmd.Dir = dir
	detach(cmd)
	cmd.SysProcAttr.CmdLine = syscall.EscapeArg(cmd.Path) + " /S /D=" + dir
	return cmd.Run()
}

func launch(target, _ string, args ...string) error {
	cmd := exec.Command(target, args...)
	cmd.Dir = filepath.Dir(target)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	return nil
}
