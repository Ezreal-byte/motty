package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, test := range []struct {
		current, latest string
		want            bool
	}{
		{"0.1.0", "0.1.1", true}, {"0.1.0", "0.2", true}, {"0.9.0", "0.10.0", true}, {"1.0", "1.0.0", false},
		{"0.2.0", "0.1.9", false}, {"v0.1.0", "v0.1.0", false}, {"0.1.0", "next", false}, {"dev", "1.0.0", false},
		{"1.0.0", "1.0.0.1", false},
	} {
		if got := Newer(test.current, test.latest); got != test.want {
			t.Errorf("Newer(%q, %q) = %v", test.current, test.latest, got)
		}
	}
}

func TestCurrentMatchesTheManifest(t *testing.T) {
	data, err := os.ReadFile("../../viva.toml")
	if os.IsNotExist(err) {
		t.Skip("the manifest is not next to this test (it was built elsewhere)")
	}
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`).FindSubmatch(data)
	if match == nil || string(match[1]) != Current {
		t.Fatalf("viva.toml version %q, update.Current %q", match, Current)
	}
}

func serve(body string, status int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			status = http.StatusForbidden
		}
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
}

func TestLatestReadsTheFeed(t *testing.T) {
	server := serve(`{"tag_name":"v0.3.0","html_url":"https://github.com/t8y2/motty/releases/tag/v0.3.0"}`, 200)
	defer server.Close()
	release, err := Latest(context.Background(), server.Client(), server.URL)
	if err != nil || release.Version != "0.3.0" || release.URL != "https://github.com/t8y2/motty/releases/tag/v0.3.0" {
		t.Fatalf("%+v %v", release, err)
	}
}

func TestLatestRefusesWhatItCannotTrust(t *testing.T) {
	for name, body := range map[string]string{
		"bad version": `{"tag_name":"nightly","html_url":"https://x/y"}`,
		"prerelease":  `{"tag_name":"v1.0.0","html_url":"https://x/y","prerelease":true}`,
		"draft":       `{"tag_name":"v1.0.0","html_url":"https://x/y","draft":true}`,
		"plain http":  `{"tag_name":"v1.0.0","html_url":"http://x/y"}`,
		"javascript":  `{"tag_name":"v1.0.0","html_url":"javascript:alert(1)"}`,
		"not json":    `<html>`,
	} {
		server := serve(body, 200)
		if _, err := Latest(context.Background(), server.Client(), server.URL); err == nil {
			t.Errorf("%s was accepted", name)
		}
		server.Close()
	}
	missing := serve(`{"message":"Not Found"}`, 404)
	defer missing.Close()
	if _, err := Latest(context.Background(), missing.Client(), missing.URL); !errors.Is(err, ErrNoRelease) {
		t.Errorf("a 404 gave %v, want ErrNoRelease", err)
	}
	server := serve(`{}`, 500)
	defer server.Close()
	if _, err := Latest(context.Background(), server.Client(), server.URL); err == nil {
		t.Error("an error status was accepted")
	}
}
