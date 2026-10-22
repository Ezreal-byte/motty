// Package update asks a release feed whether a newer version exists. It only
// reads: telling the user and opening the download page is the caller's job,
// and nothing here replaces the running program.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Current is the version of this build. A test keeps it equal to viva.toml's.
const Current = "0.1.0"

// Feed is the GitHub release the project publishes its builds to.
const Feed = "https://api.github.com/repos/t8y2/motty/releases/latest"

// ErrNoRelease means the feed has no published release, which is also what a
// private repository looks like to an anonymous request.
var ErrNoRelease = errors.New("update: no published release was found")

// Release is the newest published version.
type Release struct {
	Version string // without a leading v
	URL     string // the page to download it from
}

const maxFeedBytes = 1 << 20

// Latest reads the feed. The client decides timeouts and proxies; nil uses one
// with a 15 second timeout and the environment's proxy.
func Latest(ctx context.Context, client *http.Client, feed string) (Release, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, feed, nil)
	if err != nil {
		return Release{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "motty/"+Current)
	response, err := client.Do(request)
	if err != nil {
		return Release{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return Release{}, ErrNoRelease
	}
	if response.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("update: the release feed answered %s", response.Status)
	}
	var body struct {
		Tag        string `json:"tag_name"`
		URL        string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxFeedBytes)).Decode(&body); err != nil {
		return Release{}, fmt.Errorf("update: reading the release feed: %w", err)
	}
	version := strings.TrimPrefix(strings.TrimSpace(body.Tag), "v")
	if _, ok := parse(version); !ok || body.Draft || body.Prerelease {
		return Release{}, errors.New("update: the release feed has no usable version")
	}
	// The page is opened in the user's browser; only a web address is trusted.
	if !strings.HasPrefix(body.URL, "https://") {
		return Release{}, errors.New("update: the release page is not an https address")
	}
	return Release{Version: version, URL: body.URL}, nil
}

// Newer reports whether latest is a later version than current. Either that is
// not a plain dotted number is "no".
func Newer(current, latest string) bool {
	a, okA := parse(strings.TrimPrefix(current, "v"))
	b, okB := parse(strings.TrimPrefix(latest, "v"))
	if !okA || !okB {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return b[index] > a[index]
		}
	}
	return false
}

func parse(version string) (parts [3]int, ok bool) {
	fields := strings.Split(version, ".")
	if len(fields) == 0 || len(fields) > 3 {
		return parts, false
	}
	for index, field := range fields {
		number, err := strconv.Atoi(field)
		if err != nil || number < 0 || number > 1<<20 {
			return parts, false
		}
		parts[index] = number
	}
	return parts, true
}
