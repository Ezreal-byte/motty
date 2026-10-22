package workbench

import (
	"fmt"

	"motty/internal/term"
)

// terminalLinkLimit bounds the target strings retained after a pointer enters
// a terminal. Detection stays dormant until pointer interaction asks for it.
const terminalLinkLimit = 8192

func (state *State) openTerminalLink(target string) {
	if err := state.options.OpenURL(target); err != nil {
		state.status = fmt.Sprintf(state.messages.StatusOpenURLFailedFmt, err)
	}
}

type terminalLinks struct {
	items    []term.Link
	version  uint64
	valid    bool
	hovered  int // index in items plus one; zero means none
	pressed  term.Link
	pressing bool
}

func (links *terminalLinks) hover(screen *term.Terminal, position term.BufferPosition) bool {
	changed := links.refresh(screen)
	hovered := 0
	for index := range links.items {
		if terminalLinkContains(links.items[index], position) {
			hovered = index + 1
			break
		}
	}
	if hovered != links.hovered {
		links.hovered = hovered
		changed = true
	}
	return changed
}

func (links *terminalLinks) press(screen *term.Terminal, position term.BufferPosition) bool {
	changed := links.hover(screen, position)
	links.pressing = links.hovered > 0
	if links.pressing {
		links.pressed = links.items[links.hovered-1]
	} else {
		links.pressed = term.Link{}
	}
	return changed
}

func (links *terminalLinks) release(screen *term.Terminal, position term.BufferPosition) (string, bool, bool) {
	changed := links.hover(screen, position)
	pressed, pressing := links.pressed, links.pressing
	links.pressed = term.Link{}
	links.pressing = false
	if !pressing || links.hovered == 0 || links.items[links.hovered-1] != pressed {
		return "", false, changed
	}
	return pressed.Target, true, changed
}

func (links *terminalLinks) clear() bool {
	changed := links.hovered != 0
	links.hovered = 0
	links.pressed = term.Link{}
	links.pressing = false
	return changed
}

func (links *terminalLinks) hoveredLink(screen *term.Terminal) (term.Link, bool) {
	if !links.valid || links.version != screen.Version() || links.hovered <= 0 || links.hovered > len(links.items) {
		return term.Link{}, false
	}
	return links.items[links.hovered-1], true
}

func (links *terminalLinks) refresh(screen *term.Terminal) bool {
	version := screen.Version()
	if links.valid && links.version == version {
		return false
	}
	changed := links.hovered != 0
	clear(links.items)
	links.items, _ = screen.Links(links.items[:0], terminalLinkLimit)
	links.version = version
	links.valid = true
	links.hovered = 0
	links.pressed = term.Link{}
	links.pressing = false
	return changed
}

func terminalLinkContains(link term.Link, position term.BufferPosition) bool {
	return !terminalBufferPositionLess(position, link.Start) && terminalBufferPositionLess(position, link.End)
}

func terminalBufferPositionLess(left, right term.BufferPosition) bool {
	return left.Line < right.Line || left.Line == right.Line && left.Column < right.Column
}

func terminalLinkPosition(screen *term.Terminal, position term.Position) term.BufferPosition {
	return term.BufferPosition{Line: screen.ViewLine(position.Row), Column: position.Column}
}
