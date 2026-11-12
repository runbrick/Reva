package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestCurrentMatchesManifest(t *testing.T) {
	data, err := os.ReadFile("../../viva.toml")
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`).FindSubmatch(data)
	if len(match) != 2 || string(match[1]) != Current {
		t.Fatalf("manifest version %q, current %q", match, Current)
	}
}

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		current, latest string
		want            bool
	}{
		{"0.2.0", "0.2.1", true}, {"v0.2.0", "v0.3.0", true},
		{"0.9.0", "0.10.0", true}, {"1.0.0", "1.0.0", false},
		{"1.0.0", "0.9.0", false}, {"dev", "2.0.0", false},
		{"1.0.0", "1.0.0.1", false}, {"1.0.0", "1.01.0", false},
	} {
		if got := Newer(tc.current, tc.latest); got != tc.want {
			t.Errorf("Newer(%q, %q) = %t, want %t", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestLatestReadsReleaseAndHandlesMissingRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "Reqo/"+Current {
			t.Error("missing user agent")
		}
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"tag_name":"v0.3.0","html_url":"https://github.com/runbrick/Reva/releases/tag/v0.3.0"}`))
	}))
	defer server.Close()
	release, err := Latest(context.Background(), server.Client(), server.URL)
	if err != nil || release.Version != "0.3.0" || !Newer(Current, release.Version) {
		t.Fatalf("release %+v, error %v", release, err)
	}
	_, err = Latest(context.Background(), server.Client(), server.URL+"/missing")
	if !errors.Is(err, ErrNoRelease) {
		t.Fatalf("missing release error %v", err)
	}
}

func TestLatestRejectsUnsafeAndUnpublishedReleases(t *testing.T) {
	for name, body := range map[string]string{
		"draft":      `{"tag_name":"v0.3.0","draft":true,"html_url":"https://github.com/runbrick/Reva/releases/tag/v0.3.0"}`,
		"prerelease": `{"tag_name":"v0.3.0","prerelease":true,"html_url":"https://github.com/runbrick/Reva/releases/tag/v0.3.0"}`,
		"bad tag":    `{"tag_name":"nightly","html_url":"https://github.com/runbrick/Reva/releases/tag/nightly"}`,
		"other repo": `{"tag_name":"v0.3.0","html_url":"https://github.com/other/repo/releases/tag/v0.3.0"}`,
		"unsafe url": `{"tag_name":"v0.3.0","html_url":"javascript:alert(1)"}`,
		"too large":  strings.Repeat("x", 1<<20+10),
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(body)) }))
			defer server.Close()
			if _, err := Latest(context.Background(), server.Client(), server.URL); err == nil {
				t.Fatal("invalid release was accepted")
			}
		})
	}
}
