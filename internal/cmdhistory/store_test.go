package cmdhistory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sub", "history.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store, path
}

func TestStoreCountsRepeatsAndSeparatesHostsFromTheGlobalView(t *testing.T) {
	store, _ := openStore(t)
	base := time.Now()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(store.Record("a", "ls", base))
	must(store.Record("a", "pwd", base.Add(time.Second)))
	must(store.Record("b", "ls", base.Add(2*time.Second)))
	must(store.Record("a", "ls", base.Add(3*time.Second)))

	hostA, err := store.List("a")
	must(err)
	if len(hostA) != 2 || hostA[0].Command != "ls" || hostA[0].Count != 2 || hostA[1].Command != "pwd" {
		t.Fatalf("host a = %+v", hostA)
	}
	all, err := store.List("")
	must(err)
	if len(all) != 2 || all[0].Command != "ls" || all[0].Count != 3 {
		t.Fatalf("all hosts = %+v", all)
	}

	must(store.Delete("", "ls"))
	if remaining, _ := store.List(""); len(remaining) != 1 || remaining[0].Command != "pwd" {
		t.Fatalf("after deleting ls everywhere: %+v", remaining)
	}
}

func TestStoreKeepsTheMostRecentPerHost(t *testing.T) {
	store, _ := openStore(t)
	base := time.Now()
	// Fill the host in one transaction, then let Record do the pruning.
	tx, _ := store.db.Begin()
	for i := 0; i < MaxPerHost+24; i++ {
		tx.Exec(`INSERT INTO commands (host, command, used, count) VALUES ('a', ?, ?, 1)`, fmt.Sprintf("cmd %d", i), base.Add(time.Duration(i)*time.Millisecond).UnixMilli())
	}
	tx.Commit()
	if err := store.Record("a", fmt.Sprintf("cmd %d", MaxPerHost+24), base.Add(time.Duration(MaxPerHost+24)*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	store.Record("b", "other", base)
	entries, _ := store.List("a")
	if len(entries) != MaxPerHost || entries[0].Command != fmt.Sprintf("cmd %d", MaxPerHost+24) || entries[len(entries)-1].Command != "cmd 25" {
		t.Fatalf("kept %d, newest %q, oldest %q", len(entries), entries[0].Command, entries[len(entries)-1].Command)
	}
	if other, _ := store.List("b"); len(other) != 1 {
		t.Fatal("another host lost commands to the cap")
	}
}

func TestClearLeavesNoTextInTheFile(t *testing.T) {
	store, path := openStore(t)
	store.Record("a", "kubectl rollout restart deployment/needle-in-file", time.Now())
	store.Record("b", "other", time.Now())
	if err := store.ForgetHost("a"); err != nil {
		t.Fatal(err)
	}
	if n, _ := store.Len(); n != 1 {
		t.Fatalf("len after forgetting a host = %d", n)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "needle-in-file") {
		t.Fatal("a forgotten command is still in the file")
	}
	if err := store.Clear(); err != nil {
		t.Fatal(err)
	}
	if n, _ := store.Len(); n != 0 {
		t.Fatal("clear left commands")
	}
	data, _ = os.ReadFile(path)
	if strings.Contains(string(data), "other") {
		t.Fatal("a cleared command is still in the file")
	}
	if info, _ := os.Stat(path); info.Mode().Perm()&0o077 != 0 && os.PathSeparator == '/' {
		t.Fatalf("history file mode = %v", info.Mode())
	}
}
