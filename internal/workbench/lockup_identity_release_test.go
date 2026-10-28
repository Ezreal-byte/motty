//go:build !dev

package workbench

import (
	"testing"

	"motty/internal/buildinfo"
)

func TestReleaseLockupKeepsTheOriginalWordmark(t *testing.T) {
	state := newTestState(t, testOptions(t))
	_, texts := paintedTexts(state)
	if buildinfo.ProductName != "Motty" || containsExact(texts, "Motty Dev") {
		t.Fatalf("sidebar title = %q, painted texts = %q", buildinfo.ProductName, texts)
	}
}
