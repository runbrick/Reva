// Package update checks the published GitHub release without changing the running app.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const Current = "0.2.0"
const Feed = "https://api.github.com/repos/runbrick/Reva/releases/latest"

var ErrNoRelease = errors.New("no published release")

type Release struct {
	Version string
	URL     string
}

// Latest reads only a bounded GitHub response. The caller chooses when to check.
func Latest(ctx context.Context, client *http.Client, feed string) (Release, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Reqo/"+Current)
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return Release{}, ErrNoRelease
	}
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("GitHub release API: %s", resp.Status)
	}
	var body struct {
		Tag        string `json:"tag_name"`
		URL        string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&body); err != nil {
		return Release{}, fmt.Errorf("GitHub release API: %w", err)
	}
	version := strings.TrimPrefix(body.Tag, "v")
	page, err := url.Parse(body.URL)
	if _, valid := parse(version); !valid || body.Draft || body.Prerelease || err != nil ||
		page.Scheme != "https" || !strings.EqualFold(page.Host, "github.com") ||
		page.Path != "/runbrick/Reva/releases/tag/"+body.Tag || page.RawQuery != "" || page.Fragment != "" {
		return Release{}, errors.New("GitHub release API: invalid release")
	}
	return Release{Version: version, URL: body.URL}, nil
}

func Newer(current, latest string) bool {
	a, okA := parse(strings.TrimPrefix(current, "v"))
	b, okB := parse(strings.TrimPrefix(latest, "v"))
	if !okA || !okB {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return b[i] > a[i]
		}
	}
	return false
}

func parse(version string) ([3]int, bool) {
	var parts [3]int
	fields := strings.Split(version, ".")
	if len(fields) != 3 {
		return parts, false
	}
	for i, field := range fields {
		if field == "" || (len(field) > 1 && field[0] == '0') {
			return parts, false
		}
		n, err := strconv.Atoi(field)
		if err != nil || n < 0 || n > 1<<20 {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}
