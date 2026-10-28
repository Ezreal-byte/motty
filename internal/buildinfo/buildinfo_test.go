package buildinfo

import "testing"

func TestProductNameMatchesTheBuildTag(t *testing.T) {
	want := "Motty"
	if Development {
		want = "Motty Dev"
	}
	if ProductName != want || Development != developmentBuild {
		t.Fatalf("ProductName = %q, want %q; Development = %t, want %t", ProductName, want, Development, developmentBuild)
	}
}
