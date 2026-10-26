//go:build !dev

package workbench

import "github.com/vivaui/viva/ui"

// devOnlyShellHits is empty: a release build declares no development controls.
func devOnlyShellHits() []ui.ID { return nil }
