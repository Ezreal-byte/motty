//go:build darwin || windows || linux

package main

import (
	"reflect"
	"testing"
)

func TestLaunchArgumentsDropWhatTheSystemAdds(t *testing.T) {
	rest, background := launchArguments([]string{"-psn_0_12345", "--background", "ssh://h/", "user@h"})
	if !background || !reflect.DeepEqual(rest, []string{"ssh://h/", "user@h"}) {
		t.Fatalf("rest %q background %v", rest, background)
	}
	if rest, background := launchArguments(nil); background || len(rest) != 0 {
		t.Fatalf("rest %q background %v", rest, background)
	}
}

func TestADataDirectoryIsACopyOfItsOwn(t *testing.T) {
	if instanceID("") != applicationID || instanceID("/a") == instanceID("/b") || instanceID("/a") == applicationID {
		t.Fatalf("ids %q %q %q", instanceID(""), instanceID("/a"), instanceID("/b"))
	}
}
