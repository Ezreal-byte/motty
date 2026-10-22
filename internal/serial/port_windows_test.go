//go:build windows

package serial

import "testing"

func TestAMissingPortFailsToOpenAndListingWorks(t *testing.T) {
	if _, err := Start(Config{Device: "COM98", Settings: Default}); err == nil {
		t.Fatal("opened COM98")
	}
	for _, port := range Ports() {
		if comNumber(port.Device) == 0 {
			t.Errorf("odd port %+v", port)
		}
	}
}

func TestComNumbersSortNumerically(t *testing.T) {
	if comNumber("COM10") != 10 || comNumber("COM3") != 3 || comNumber("COM") != 0 {
		t.Fatal("comNumber")
	}
}
