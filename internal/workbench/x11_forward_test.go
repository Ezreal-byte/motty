package workbench

import "testing"

func TestForwardX11IsOffUntilSwitchedOnAndSurvivesARestart(t *testing.T) {
	state := newTestState(t, testOptions(t))
	state.Click(hitAddHost)
	state.editor.label.Insert("Display box")
	state.editor.address.Insert("display.example.com")
	state.Click(hitDialogSubmit)
	find := func(s *State) Host {
		for _, host := range s.hosts {
			if host.Label == "Display box" {
				return host
			}
		}
		t.Fatal("host missing")
		return Host{}
	}
	if find(state).ForwardX11 {
		t.Fatal("a new host forwards X11")
	}

	state.selectedHostID = find(state).ID
	state.openEditHost()
	state.Click(hitDialogForwardX11)
	if !state.editor.forwardX11 {
		t.Fatal("the switch did not turn on")
	}
	state.Click(hitDialogSubmit)
	if !find(state).ForwardX11 {
		t.Fatal("the choice was not saved")
	}
	again := New(state.options)
	defer again.Close()
	if !find(again).ForwardX11 {
		t.Fatal("the choice did not survive a restart")
	}
}
