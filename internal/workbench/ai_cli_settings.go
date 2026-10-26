package workbench

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
	"motty/internal/ai/store"
)

func (state *State) paintAICLIProviderForm(canvas *ui.Canvas, viewport ui.Rect) {
	cfg, m, theme := &state.ai.cfg, state.messages.AISettings, state.Theme()
	title, submit := m.FormAddTitle, m.FormAdd
	height := float32(182)
	if cfg.editID != "" {
		title, submit, height = m.FormEditTitle, m.FormSave, 122
		title += " (" + aiPresets[aiPresetOf(store.Provider{Kind: cfg.kind})].label + ")"
	}
	dialog := components.BeginDialog(canvas, theme, viewport, components.DialogOptions{
		Layer: dialogLayer, Scope: dialogScope, DismissID: hitAISetFormDismiss, CloseID: hitAISetFormClose,
		InitialFocusID: hitAISetFormName, ContentSize: ui.Size{Width: 500}, FitBody: true, BodyHeight: withError(height, cfg.formErr),
	})
	layout := dialog.Begin(title, m.CLIFormDescription)
	var preset, name, path flex.Box
	f := state.settingsRow.Begin(canvas, theme, layout.Body)
	f.Column().Fill().PadSides(2, 4, 0, 4).Children(func() {
		if cfg.editID == "" {
			preset = f.Slot().Height(60)
		}
		name = f.Slot().Height(60)
		path = f.Slot().Height(60)
		if cfg.formErr != "" {
			f.Text(cfg.formErr).FontSize(metrics.FontSizeS).Color(state.pal.danger).LineHeight(18)
		}
	})
	f.End()
	if cfg.editID == "" {
		state.paintAIPresetField(canvas, preset.Rect())
	}
	state.aiFormField(canvas, hitAISetFormName, m.FormName, m.FormNamePH, "", false, &cfg.name, name.Rect())
	state.aiFormField(canvas, hitAISetFormCLIPath, m.CLIPath, m.CLIPathDefault, "", false, &cfg.cliPath, path.Rect())
	dialog.Actions(layout, components.ButtonAction{ID: hitAISetFormCancel, Label: state.messages.Cancel, Variant: components.ButtonSecondary}, components.ButtonAction{ID: hitAISetFormSubmit, Label: submit})
	dialog.End()
}
