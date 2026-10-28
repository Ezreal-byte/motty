package update

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type acceptanceTransport func(*http.Request) (*http.Response, error)

func (f acceptanceTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestAcceptanceChannelPinsThePackageToTheFork(t *testing.T) {
	client := &http.Client{Transport: acceptanceTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != MirrorFeed {
			t.Fatalf("unexpected feed: %s", request.URL)
		}
		body := `{"version":"0.1.902","tag":"v0.1.902","source":"https://github.com/Ezreal-byte/motty/releases/tag/v0.1.902","files":[{"name":"motty-windows-amd64-setup.exe","size":100,"sha256":"` + strings.Repeat("a", 64) + `","signature":"c2ln"}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	release, err := Check(context.Background(), client, "windows", "amd64")
	want := "https://github.com/Ezreal-byte/motty/releases/download/v0.1.902/motty-0.1.902-windows-amd64-setup.exe"
	if err != nil || release.Asset.URL != want || !release.Asset.Has() || !release.Asset.Signed() || !Newer("0.1.901", release.Version) {
		t.Fatalf("acceptance upgrade unavailable: %+v, %v", release, err)
	}
}
