//go:build windows || darwin || linux

package workbench

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/components"
)

func TestSFTPWindowLocalEditorUsesOriginalPathAndGlobalErrors(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "open", true: "error"}[fail], func(t *testing.T) {
			state, w, _, host, root := dualSFTPFixture(t, 1)
			view := host.windows[0].view
			opened := ""
			state.options.OpenPath = func(location string) error {
				opened = location
				if fail {
					return errors.New("editor unavailable")
				}
				return nil
			}
			if err := view.RightClickLabel("中文上传.yml"); err != nil {
				t.Fatal(err)
			}
			if err := view.Rest(); err != nil {
				t.Fatal(err)
			}
			dualClickID(t, view, sftpWindowMenuActions.At(sftpWindowActionIndex(hitSFTPEditExternal)))
			if opened != filepath.Join(root, "中文上传.yml") || len(state.edits.list) != 0 || state.edits.opening.active || len(w.view.worker.Transfers(nil)) != 0 {
				t.Fatal("dual-pane local edit started remote editing or opened a copy")
			}
			if fail && (w.view.errorText != "" || len(w.notices.items) != 1 || w.notices.items[0].kind != components.ToastError) {
				t.Fatal("dual-pane editor failure did not appear in its window's global notices")
			}
		})
	}
}
