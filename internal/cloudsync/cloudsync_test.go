package cloudsync

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func TestRemoteHTTPRequiresExplicitConsent(t *testing.T) {
	for _, raw := range []string{"http://192.168.1.20:8080/dav", "http://backup.lan/dav", "http://[fd00::20]/dav"} {
		address, _ := url.Parse(raw)
		if err := CheckAddress(address, false); !errors.Is(err, ErrInsecure) {
			t.Errorf("unapproved %s: %v", raw, err)
		}
		if err := CheckAddress(address, true); err != nil {
			t.Errorf("approved %s: %v", raw, err)
		}
	}
	for _, raw := range []string{"https://backup.lan/dav", "http://localhost:8080/dav", "http://127.0.0.1/dav", "http://[::1]/dav"} {
		address, _ := url.Parse(raw)
		if err := CheckAddress(address, false); err != nil {
			t.Errorf("secure or local %s: %v", raw, err)
		}
	}
	address, _ := url.Parse("ftp://backup.lan/dav")
	if CheckAddress(address, true) == nil {
		t.Fatal("HTTP consent allowed a different protocol")
	}
}

func TestHTTPConsentDoesNotAllowCrossHostRedirects(t *testing.T) {
	first, _ := http.NewRequest(http.MethodGet, "https://backup.lan/dav", nil)
	downgrade, _ := http.NewRequest(http.MethodGet, "http://backup.lan/dav", nil)
	other, _ := http.NewRequest(http.MethodGet, "http://other.lan/dav", nil)
	for _, allowed := range []bool{false, true} {
		client := HTTPClient(nil, allowed)
		if err := client.CheckRedirect(other, []*http.Request{first}); !errors.Is(err, http.ErrUseLastResponse) {
			t.Fatalf("cross-host redirect (consent=%v): %v", allowed, err)
		}
		err := client.CheckRedirect(downgrade, []*http.Request{first})
		if allowed && err != nil || !allowed && !errors.Is(err, ErrInsecure) {
			t.Fatalf("HTTPS downgrade (consent=%v): %v", allowed, err)
		}
	}
}
