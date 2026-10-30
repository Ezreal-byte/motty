//go:build darwin || windows || linux

package workbench

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"motty/internal/i18n"
)

func TestForwardPrerequisiteToastThroughNativeDriver(t *testing.T) {
	for _, language := range []i18n.Language{i18n.Chinese, i18n.English} {
		for _, scale := range []float64{1, 1.25, 1.5, 1.75, 2} {
			t.Run(fmt.Sprintf("%s/%g", language.String(), scale), func(t *testing.T) {
				_, options := newWorld(t)
				state := newTestState(t, options)
				state.hosts = nil
				state.setLanguage(language)
				state.Click(hitPortForwarding)
				view, err := headless.Open(app.Config{Title: "Action feedback", Width: 1100, Height: 760, Tree: Tree(), Paint: state.Paint, OnResize: state.Resize, OnPointer: state.Pointer, OnKey: state.Key}, nil, headless.Options{Scale: scale})
				if err != nil {
					t.Fatal(err)
				}
				defer view.Close()
				if err = view.ClickLabel(state.messages.ForwardAdd); err != nil {
					t.Fatal(err)
				}
				if err = view.Wait(600 * time.Millisecond); err != nil {
					t.Fatal(err)
				}
				if len(state.fileNotices.items) != 1 {
					t.Fatal("real click did not show the prerequisite notice")
				}
				item := state.fileNotices.items[0]
				closeHit, ok := findSnapshotHit(view.Snapshot(), item.id)
				if !ok || closeHit.Bounds.X < 0 || closeHit.Bounds.X+closeHit.Bounds.Width > 1100 || closeHit.Bounds.Y < 0 || closeHit.Bounds.Y+closeHit.Bounds.Height > 760 {
					t.Fatalf("toast dismiss control is clipped: %+v", closeHit)
				}
				found := false
				for _, text := range view.Snapshot().Texts {
					if text.Value == state.messages.ErrForwardHostRequired {
						found = true
					}
				}
				if !found {
					t.Fatal("prerequisite text is not painted")
				}
				if dir := os.Getenv("MOTTY_ACTION_NOTICE_SCREENSHOTS"); dir != "" && scale == 1.25 {
					if err = view.Screenshot(filepath.Join(dir, "action-notice-"+language.String()+".png")); err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	}
}
