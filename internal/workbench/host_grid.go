package workbench

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// The hosts page shows its hosts as a list or as cards. Both read the same rows
// (hostTreeRows), use the same ids for selecting, checking and opening a host and
// share one scroll position; only the layout differs.

type hostViewMode uint8

const (
	hostViewList hostViewMode = iota
	hostViewGrid
)

// hostGridHits is the place of the controls this file paints.
var hostGridHits = tree.Feature("host-grid")

var (
	hitHostViewList = tree.ID(hostGridHits, "host-view-list")
	hitHostViewGrid = tree.ID(hostGridHits, "host-view-grid")
)

const (
	hostCardMinWidth = float32(280)
	hostCardHeight   = float32(64)
	hostCardGap      = float32(12)
	hostGridHeader   = float32(42)
)

func hostViewFromString(value string) hostViewMode {
	if value == "grid" {
		return hostViewGrid
	}
	return hostViewList
}

func (mode hostViewMode) saved() string {
	if mode == hostViewGrid {
		return "grid"
	}
	return ""
}

func (state *State) clickHostView(id ui.ID) bool {
	var mode hostViewMode
	switch id {
	case hitHostViewList:
		mode = hostViewList
	case hitHostViewGrid:
		mode = hostViewGrid
	default:
		return false
	}
	if state.activeSection == sectionHosts && state.modal == modalNone && state.hostView != mode {
		state.hostView, state.hostsScroll = mode, 0
		state.save()
	}
	return true
}

// paintHostViewToggle draws the two view buttons from x, centred on the line
// center. The caller is paintHostsHeader, which hands over the line, not a box.
func (state *State) paintHostViewToggle(canvas *ui.Canvas, x, center float32) {
	m := state.messages.Tools
	buttons := [...]struct {
		id    ui.ID
		icon  uint8
		label string
		mode  hostViewMode
	}{
		{hitHostViewList, iconRows2, m.HostViewList, hostViewList},
		{hitHostViewGrid, iconHome, m.HostViewGrid, hostViewGrid},
	}
	var slots [len(buttons)]flex.Box
	f := state.hostToggleLayout.Begin(canvas, state.Theme(), ui.Rect{X: x, Y: center, Width: 54})
	f.Row().Fill().Gap(2).Items(flex.Center).Children(func() {
		for index := range buttons {
			slots[index] = f.Slot().Size(26, 26)
		}
	})
	f.End()
	for index, button := range buttons {
		style := iconButtonQuiet
		if state.hostView == button.mode {
			style = iconButtonActive
		}
		state.paintIconButton(canvas, button.id, button.icon, slots[index].Rect(), button.label, false, style, state.pal.workspace)
	}
}

// hostGridColumns is how many cards fit across width.
func hostGridColumns(width float32) int {
	return max(int((width+hostCardGap)/(hostCardMinWidth+hostCardGap)), 1)
}

// hostGridHeight is the height of the whole card layout at width.
func (state *State) hostGridHeight(width float32) float32 {
	columns := hostGridColumns(width)
	var height float32
	run := 0
	flush := func() {
		if run > 0 {
			height += float32((run+columns-1)/columns) * (hostCardHeight + hostCardGap)
			run = 0
		}
	}
	for _, entry := range state.hostTreeRows {
		if entry.Kind == hostTreeGroup {
			flush()
			height += hostGridHeader
		} else {
			run++
		}
	}
	flush()
	return height
}

// hostGridItem is a card or a group header that is on screen, with the box
// the layout gave it.
type hostGridItem struct {
	box   flex.Box
	entry int // the index into hostTreeRows
}

// paintHostGrid draws the cards inside bounds, scrolled by hostsScroll. Only
// what is on screen is declared: the rows above the window are skipped by their
// known height and the layout is shifted up by what is left of the first one.
func (state *State) paintHostGrid(canvas *ui.Canvas, bounds ui.Rect) {
	columns := hostGridColumns(bounds.Width)
	rows := state.hostTreeRows
	items := state.hostGridItems[:0]
	y := bounds.Y - state.hostsScroll
	bottom := bounds.Y + bounds.Height
	shift, first := float32(0), true
	f := state.hostGridLayout.Begin(canvas, state.Theme(), bounds)
	list := f.Column().FillWidth().Children(func() {
		for index := 0; index < len(rows) && y < bottom; {
			if rows[index].Kind == hostTreeGroup {
				// Its text sits on the same line as a group row's in the list view.
				if y+hostGridHeader > bounds.Y {
					if first {
						shift, first = y-bounds.Y, false
					}
					items = append(items, hostGridItem{f.Slot().Height(hostGridHeader-8).FillWidth().MarginSides(4, 0, 4, 0).Shrink(0), index})
				}
				y += hostGridHeader
				index++
				continue
			}
			end := index
			for end < len(rows) && end-index < columns && rows[end].Kind == hostTreeHost {
				end++
			}
			if y+hostCardHeight >= bounds.Y {
				if first {
					shift, first = y-bounds.Y, false
				}
				// Every row has all its columns, so the last card is no wider than the rest.
				f.Row().Height(hostCardHeight).Gap(hostCardGap).MarginSides(0, 0, hostCardGap, 0).Shrink(0).Children(func() {
					for column := 0; column < columns; column++ {
						card := f.Slot().Grow(1).Basis(0)
						if index+column < end {
							items = append(items, hostGridItem{card, index + column})
						}
					}
				})
			}
			y += hostCardHeight + hostCardGap
			index = end
		}
	})
	list.MarginSides(shift, 0, 0, 0)
	f.End()
	state.hostGridItems = items

	for _, item := range items {
		if rows[item.entry].Kind == hostTreeGroup {
			state.paintHostGridHeader(canvas, item.box.Rect(), rows[item.entry].GroupIndex)
		} else {
			state.paintHostCard(canvas, item.box.Rect(), state.hosts[rows[item.entry].HostIndex])
		}
	}
}

// paintHostGridHeader draws the row that opens or closes a group of cards.
func (state *State) paintHostGridHeader(canvas *ui.Canvas, header ui.Rect, groupIndex int) {
	pal, group := state.pal, state.hostGroups[groupIndex]
	label := state.hostGroupLabel(group)
	chevron := iconChevronRight
	if group.Expanded || state.visibleQuery != "" {
		chevron = iconChevronDown
	}
	var icon flex.Box
	f := state.partLayout.Begin(canvas, state.Theme(), header)
	f.Row().Fill().PadSides(0, 0, 0, 2).Gap(8).Items(flex.Center).
		Radius(radius.control).HoverBackground(pal.surfaceSoft).
		HitWith(hitHostGroupOpenList.At(groupIndex), label, ui.HitOptions{Role: ui.RoleButton, Expanded: group.Expanded}).
		Children(func() {
			icon = f.Slot().Size(16, 16)
			f.Row().Gap(6).Items(flex.Center).Children(func() {
				f.Text(label).FontSize(metrics.FontSizeS).Weight(ui.WeightSemibold).Color(pal.text).LineHeight(23)
				f.Text(group.Count).FontSize(metrics.FontSizeS).Weight(ui.WeightMedium).Color(pal.faint).LineHeight(21)
			})
		})
	f.End()
	paintIcon(canvas, uint8(chevron), icon.Rect(), pal.muted)
}

func (state *State) paintHostCard(canvas *ui.Canvas, card ui.Rect, host Host) {
	pal := state.pal
	id := hitHostList.At(int(host.ID))
	selected := host.ID == state.selectedHostID
	switch {
	case selected:
		canvas.RoundedBorder(card, radius.surface, 1.5, pal.accent, pal.accentSoft)
	case canvas.Hovered() == id:
		canvas.RoundedBorder(card, radius.surface, 1, withAlpha(pal.line, 0.9), pal.surfaceSoft)
	default:
		canvas.RoundedBorder(card, radius.surface, 1, withAlpha(pal.line, 0.8), pal.surface)
	}
	canvas.HitWithOptions(id, card, host.Label, ui.HitOptions{Role: ui.RoleButton, Selected: selected})
	state.hostRowEvents(canvas, id, host)
	if canvas.FocusVisible() == id {
		canvas.RoundedBorder(card, radius.surface, 1.5, pal.accent, ui.Color{})
	}

	// The avatar and the two lines of text, with the check box at the corner. The
	// connection's state is a light on the avatar's corner; its words and the
	// address are in the list view, and a card says the name and who.
	_, dot := state.hostStatus(host)
	kind, _ := state.hostLight(host)
	background := pal.surface
	if selected {
		background = pal.accentSoft
	}
	// The tags go on the second line after the user, as many whole pills as fit the
	// room left of the check box.
	subtitle := state.hostSubtitle(host)
	textWidth := card.Width - 14 - 12 - 36 - 12*2 - components.CheckboxExtent(state.Theme())
	tagCount := 0
	var tagList [maxHostTags]string
	subtitleWidth := canvas.TextWidth(subtitle, 11)
	if room := textWidth - subtitleWidth - 8; room >= 40 {
		tagCount = state.fitHostTags(canvas, host, room, &tagList)
	}
	var avatar, light flex.Box
	f := state.partLayout.Begin(canvas, state.Theme(), card)
	f.Row().Fill().PadSides(11, 12, 0, 14).Gap(12).Children(func() {
		f.Stack().Size(36, 36).MarginSides(3, 0, 0, 0).Shrink(0).Children(func() {
			avatar = f.Slot()
			if kind != lightIdle {
				light = f.Slot().Size(14, 14).Self(flex.End).MarginSides(0, -5, -5, 0)
			}
		})
		f.Column().Grow(1).Children(func() {
			f.Text(host.Label).FontSize(metrics.FontSizeM).Weight(ui.WeightSemibold).Color(pal.text).LineHeight(22)
			f.Row().Gap(8).Items(flex.Center).MarginSides(1, 0, 0, 0).Children(func() {
				f.Text(subtitle).FontSize(metrics.FontSizeS).Color(pal.muted).LineHeight(19).Width(subtitleWidth + 1).Shrink(0)
				if tagCount > 0 {
					f.Row().Gap(5).Shrink(0).Items(flex.Center).Children(func() {
						for _, tag := range tagList[:tagCount] {
							f.Row().Height(18).PadX(7).Shrink(0).Items(flex.Center).Radius(9).Background(pal.surfaceSoft).Children(func() {
								f.Text(tag).FontSize(metrics.FontSizeS).Weight(ui.WeightMedium).Color(pal.muted).Width(canvas.TextWidthWeight(tag, metrics.FontSizeS, ui.WeightMedium) + 1).Shrink(0)
							})
						}
					})
				}
			})
		})
		f.Checkbox(hitHostCheckList.At(int(host.ID)), state.hostSelected(host.ID)).Label(host.Label).MarginSides(1, 0, 0, 0)
	})
	f.End()
	state.paintHostCatalogIcon(canvas, host, avatar.Rect(), background)
	if kind != lightIdle {
		corner := light.Rect()
		canvas.Rounded(corner, corner.Width/2, background)
		paintLight(canvas, corner.X+corner.Width/2, corner.Y+corner.Height/2, kind, dot, background)
	}
}
