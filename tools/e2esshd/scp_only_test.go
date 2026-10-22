package main

import (
	"reflect"
	"testing"
)

func TestScpOnlyAllowsOnlyFixtureCommands(t *testing.T) {
	root := "/tmp/fixture"
	allowed := map[string][]string{
		`scp -f -r -p -- '/tmp/fixture/docs'`:        {"scp", "-f", "-r", "-p", "--", "/tmp/fixture/docs"},
		`scp -t -p -- 'sub dir'`:                     {"scp", "-t", "-p", "--", "/tmp/fixture/sub dir"},
		`rm -- '/tmp/fixture/it'\''s'`:               {"rm", "--", "/tmp/fixture/it's"},
		`mkdir -p -- '/tmp/fixture/new'`:             {"mkdir", "-p", "--", "/tmp/fixture/new"},
		`chmod 0640 -- '/tmp/fixture/alpha.txt'`:     {"chmod", "0640", "--", "/tmp/fixture/alpha.txt"},
		`ln -s -- '../anywhere' '/tmp/fixture/link'`: {"ln", "-s", "--", "../anywhere", "/tmp/fixture/link"},
	}
	for command, want := range allowed {
		got, ok := scpCommand(root, command)
		if !ok || !reflect.DeepEqual(got[0], want) {
			t.Errorf("%s: got %v %v, want %v", command, got, ok, want)
		}
	}
	for _, command := range []string{
		`rm -- '/etc/passwd'`,
		`scp -f -- '../outside'`,
		`scp -f -- '/tmp/fixture/../outside'`,
		`rm -- '/tmp/fixture/a'; id`,
		`rm -- $(id)`,
		`cat '/tmp/fixture/alpha.txt'`,
		`scp -S evil -f -- '/tmp/fixture/a'`,
		`ls -lAn -- '/tmp/fixture' | sh`,
		`mkdir -p -- '/tmp/fixture/a' '/etc/b'`,
	} {
		if _, ok := scpCommand(root, command); ok {
			t.Errorf("%s was allowed", command)
		}
	}
}
