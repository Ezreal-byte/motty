//go:build dev

package workbench

import "github.com/vivaui/viva/ui"

// devOnlyShellHits names the shell controls that only a development build
// declares: the sidebar entry of the component gallery.
func devOnlyShellHits() []ui.ID { return []ui.ID{hitDebug} }
