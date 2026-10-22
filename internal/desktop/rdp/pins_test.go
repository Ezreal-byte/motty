package rdp

import (
	"path/filepath"
	"testing"
)

func TestPinsRememberTheCertificateAndReplaceItWhenTheUserTrustsANewOne(t *testing.T) {
	pins := NewPins(filepath.Join(t.TempDir(), "sub", "rdp_certificates"))
	if got, err := pins.Lookup("host:3389"); got != "" || err != nil {
		t.Fatalf("empty store: %q, %v", got, err)
	}
	first, second := Fingerprint([]byte("one")), Fingerprint([]byte("two"))
	if first == second {
		t.Fatal("different certificates share a fingerprint")
	}
	for _, step := range []struct{ address, fingerprint string }{{"a:3389", first}, {"b:3389", second}, {"a:3389", second}} {
		if err := pins.Remember(step.address, step.fingerprint); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := pins.Lookup("a:3389"); got != second {
		t.Errorf("a pin after being replaced = %q, want the new one", got)
	}
	if got, _ := pins.Lookup("b:3389"); got != second {
		t.Errorf("another server's pin = %q", got)
	}
	// A new store on the same file sees what was written: it is persistent.
	if got, _ := NewPins(pins.path).Lookup("a:3389"); got != second {
		t.Errorf("reopened store = %q", got)
	}
	if err := pins.Remember("bad host", first); err == nil {
		t.Error("an address with a space was accepted")
	}
}
