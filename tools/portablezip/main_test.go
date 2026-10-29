package main

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"

	"motty/internal/datadir"
)

func TestTheArchiveHoldsTheProgramBesideThePortableFolder(t *testing.T) {
	stamp := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	var first, second bytes.Buffer
	if err := write(&first, strings.NewReader("MZ program"), stamp); err != nil {
		t.Fatal(err)
	}
	if err := write(&second, strings.NewReader("MZ program"), stamp); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("the same program and time gave different archives")
	}
	reader, err := zip.NewReader(bytes.NewReader(first.Bytes()), int64(first.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range reader.File {
		names = append(names, entry.Name)
	}
	if want := []string{"motty.exe", datadir.PortableName + "/README.txt"}; strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("entries %v, want %v", names, want)
	}
}
