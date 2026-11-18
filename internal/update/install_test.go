package update

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLatestSelectsOnlyExactOfficialPlatformAsset(t *testing.T) {
	const version = "9.1.2"
	const name = "reva-9.1.2-windows-amd64-setup.exe"
	asset := releaseAsset{Name: name, URL: releaseRoot + "download/v" + version + "/" + name, Size: 5, Digest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("setup")))}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v%s","html_url":%q,"assets":[{"name":%q,"browser_download_url":%q,"size":%d,"digest":%q}]}`,
			version, releaseRoot+"tag/v"+version, asset.Name, asset.URL, asset.Size, asset.Digest)
	}))
	defer server.Close()
	release, err := Latest(t.Context(), server.Client(), server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if got := release.offerFor("windows", "amd64", "reva.exe", setup); got.asset != asset {
		t.Fatalf("unexpected asset: %+v", got.asset)
	}
	if release.offerFor("windows", "arm64", "reva.exe", setup).Available() ||
		release.offerFor("windows", "amd64", "reva.exe", portable).Available() {
		t.Fatal("offered an incompatible asset")
	}
	for name, assets := range map[string][]releaseAsset{
		"duplicate":      {asset, asset},
		"missing digest": {{Name: asset.Name, URL: asset.URL, Size: asset.Size}},
		"untrusted URL":  {{Name: asset.Name, URL: "https://example.com/" + asset.Name, Size: asset.Size, Digest: asset.Digest}},
		"oversized":      {{Name: asset.Name, URL: asset.URL, Size: maxAssetBytes + 1, Digest: asset.Digest}},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := Release{Version: version, assets: assets}
			if candidate.offerFor("windows", "amd64", "reva.exe", setup).Available() {
				t.Fatal("offered an invalid asset")
			}
		})
	}
}

func TestDownloadVerifiesSizeAndChecksumAndRemovesFailures(t *testing.T) {
	for _, body := range []string{"package", "damaged", "short", "package with excess"} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.(http.Flusher).Flush()
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			path := filepath.Join(t.TempDir(), "update.zip")
			asset := releaseAsset{URL: server.URL, Size: int64(len("package")), Digest: fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("package")))}
			err := download(t.Context(), server.Client(), asset, path)
			if body == "package" {
				data, readErr := os.ReadFile(path)
				if err != nil || readErr != nil || string(data) != body {
					t.Fatalf("verified package = %q, errors: %v / %v", data, err, readErr)
				}
			} else if _, statErr := os.Stat(path); err == nil || !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("bad download was kept or accepted: download %v, stat %v", err, statErr)
			}
		})
	}
}

func TestDownloadRedirectsStayOnHTTPSGitHubReleaseHosts(t *testing.T) {
	for _, target := range []string{
		"https://release-assets.githubusercontent.com/package",
		"https://objects.githubusercontent.com/package",
		"https://github.com/package",
		"http://release-assets.githubusercontent.com/package",
		"https://github.com.example.test/package",
		"https://objects.githubusercontent.com:444/package",
	} {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
		if err != nil {
			t.Fatal(err)
		}
		want := strings.HasPrefix(target, "https://") && !strings.Contains(target, "example.test") && !strings.Contains(target, ":444")
		if got := assetRedirect(req, nil) == nil; got != want {
			t.Errorf("redirect %q accepted = %t, want %t", target, got, want)
		}
	}
}

func TestUnpackAppAndRejectArchiveTraversal(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		t.Run(fmt.Sprint(unsafe), func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "release.zip")
			file, err := os.Create(archive)
			if err != nil {
				t.Fatal(err)
			}
			writer := zip.NewWriter(file)
			names := []string{"Reva.app/Contents/MacOS/reva", "Reva.app/Contents/Info.plist"}
			if unsafe {
				names = append(names, "Reva.app/../outside")
			}
			for _, name := range names {
				header := &zip.FileHeader{Name: name}
				header.SetMode(0o755)
				entry, err := writer.CreateHeader(header)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.WriteString(entry, "fixture"); err != nil {
					t.Fatal(err)
				}
			}
			if err := errors.Join(writer.Close(), file.Close()); err != nil {
				t.Fatal(err)
			}
			err = unpack(t.Context(), archive, filepath.Join(dir, "staged.app"), bundle)
			if (err != nil) != unsafe {
				t.Fatalf("unpack returned %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "outside")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("archive wrote outside the bundle: %v", err)
			}
		})
	}
}

func TestReplaceRestoresPreviousProgramAfterFailure(t *testing.T) {
	for _, phase := range []string{"apply", "restart", "success"} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			target, previous := filepath.Join(dir, "reva"), filepath.Join(dir, "previous")
			if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
				t.Fatal(err)
			}
			fixtureErr := errors.New("fixture failure")
			err := replace(target, previous, func() error {
				if err := os.WriteFile(target, []byte("new"), 0o600); err != nil {
					return err
				}
				if phase == "apply" {
					return fixtureErr
				}
				return nil
			}, func() error {
				if phase == "restart" {
					return fixtureErr
				}
				return nil
			})
			want := "new"
			if phase != "success" {
				want = "old"
				if !errors.Is(err, fixtureErr) {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if data, err := os.ReadFile(target); err != nil || string(data) != want {
				t.Fatalf("program = %q, want %q; error %v", data, want, err)
			}
		})
	}
}
