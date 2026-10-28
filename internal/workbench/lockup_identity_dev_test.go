//go:build dev

package workbench

import (
	"testing"

	"motty/internal/buildinfo"
)

func TestDevelopmentLockupPaintsTheBuildProductName(t *testing.T) {
	state := newTestState(t, testOptions(t))
	_, texts := paintedTexts(state)
	if !containsExact(texts, buildinfo.ProductName) || buildinfo.ProductName != "Motty Dev" {
		t.Fatalf("sidebar title = %q, painted texts = %q", buildinfo.ProductName, texts)
	}
}
