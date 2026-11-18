package update

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maxExpandedBytes = 256 << 20

func unpack(ctx context.Context, archive, payload, kind string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer reader.Close()
	if len(reader.File) > 1024 {
		return errors.New("update archive has too many entries")
	}
	remaining := int64(maxExpandedBytes)
	for _, entry := range reader.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := strings.TrimSuffix(entry.Name, "/")
		if !fs.ValidPath(name) || strings.ContainsAny(name, `\:`) || !filepath.IsLocal(filepath.FromSlash(name)) {
			return errors.New("unsafe update archive path")
		}
		mode := entry.Mode()
		if !mode.IsDir() && !mode.IsRegular() {
			return errors.New("unsafe update archive entry")
		}
		if kind == bundle && name == "Reva.app/Contents/MacOS/reva" && (!mode.IsRegular() || mode.Perm()&0o111 == 0) {
			return errors.New("Reva application bundle has no runnable executable")
		}
		var target string
		switch {
		case kind == portable && name == "reva.exe" && mode.IsRegular():
			target = payload
		case kind == portable:
			continue
		case kind == bundle && name == "Reva.app" && mode.IsDir():
			target = payload
		case kind == bundle && strings.HasPrefix(name, "Reva.app/"):
			target = filepath.Join(payload, filepath.FromSlash(strings.TrimPrefix(name, "Reva.app/")))
		default:
			return errors.New("unexpected update archive entry")
		}
		if mode.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if entry.UncompressedSize64 > uint64(remaining) {
			return errors.New("update archive exceeds the extraction limit")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := unpackFile(entry, target); err != nil {
			return err
		}
		remaining -= int64(entry.UncompressedSize64)
	}
	executable := payload
	if kind == bundle {
		executable = filepath.Join(payload, "Contents", "MacOS", "reva")
	}
	info, err := os.Stat(executable)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return errors.New("update archive does not contain the Reva executable")
	}
	if kind == bundle {
		info, err := os.Stat(filepath.Join(payload, "Contents", "Info.plist"))
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("update archive does not contain a Reva application bundle")
		}
	}
	return nil
}

func unpackFile(entry *zip.File, target string) error {
	in, err := entry.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, entry.Mode().Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, io.LimitReader(in, int64(entry.UncompressedSize64)+1))
	return errors.Join(err, out.Close())
}
