package backup

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	UseCheapKDFForTests()
	os.Exit(m.Run())
}
