package workbench

import (
	"os"
	"testing"

	"github.com/vivaui/viva/leaks"

	"motty/internal/backup"
	"motty/internal/vault"
)

// The tests must close every shell and worker they start.
func TestMain(m *testing.M) {
	backup.UseCheapKDFForTests()
	vault.UseCheapKDFForTests()
	os.Exit(leaks.Main(m))
}
