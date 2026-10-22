package diag

import (
	"fmt"
	"strings"
	"testing"
)

func TestLogKeepsTheLastLinesWithinItsLimits(t *testing.T) {
	log := NewLog(3, 1<<20)
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(log, "line %d\n", i)
	}
	if got := strings.Join(log.Lines(), "|"); got != "line 3|line 4|line 5" {
		t.Fatalf("lines = %q", got)
	}
	// A line written in pieces is one line, and an unfinished one is not shown.
	log = NewLog(10, 1<<20)
	log.Write([]byte("par"))
	log.Write([]byte("tial\nrest"))
	if got := strings.Join(log.Lines(), "|"); got != "partial" {
		t.Fatalf("lines = %q", got)
	}
	small := NewLog(100, 20)
	for i := 0; i < 10; i++ {
		small.Write([]byte("0123456789\n"))
	}
	if n := len(small.Lines()); n > 2 {
		t.Fatalf("a 20-byte log kept %d lines", n)
	}
}

func TestRedactingNamesAddressesAndCredentials(t *testing.T) {
	var r Redactor
	r.Hide("db.internal.example.com", "host-1")
	r.Hide("internal.example.com", "domain")
	r.Hide("deploy", "user-1")
	r.Hide("22", "port") // too short to be worth replacing
	for _, test := range []struct{ in, want string }{
		{"dial db.internal.example.com: refused", "dial host-1: refused"},
		{"dial tcp 10.1.2.3:22: i/o timeout", "dial tcp <ip>:22: i/o timeout"},
		{"login deploy@bastion failed", "login <user>@bastion failed"},
		{"mail alice@corp.example", "mail <account>"},
		{"auth password=hunter2 refused", "auth password=<hidden> refused"},
		{"dial tcp [2001:db8::1]:22", "dial tcp [<ip>]:22"},
		{"2026/10/19 16:16:59 started", "2026/10/19 16:16:59 started"},
	} {
		if got := r.Apply(test.in); got != test.want {
			t.Errorf("Apply(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

func TestAReportReadsAsSections(t *testing.T) {
	var r Redactor
	r.Hide("secret.example", "host-1")
	report := NewReport(&r)
	report.Section("One")
	report.Field("target", "secret.example:2222")
	report.Fact("count", 3)
	report.Section("Two")
	report.Line("plain")
	want := "== One ==\ntarget: host-1:2222\ncount: 3\n\n== Two ==\nplain\n"
	if report.String() != want {
		t.Fatalf("report =\n%q\nwant\n%q", report.String(), want)
	}
}
