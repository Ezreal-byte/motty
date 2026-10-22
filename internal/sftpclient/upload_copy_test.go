package sftpclient

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type concurrentSink struct {
	data        bytes.Buffer
	used        bool
	concurrency int
}

func (sink *concurrentSink) Write(chunk []byte) (int, error) { return sink.data.Write(chunk) }

func (sink *concurrentSink) ReadFromWithConcurrency(source io.Reader, concurrency int) (int64, error) {
	sink.used = true
	sink.concurrency = concurrency
	return io.Copy(&sink.data, source)
}

type refusedSink struct {
	data bytes.Buffer
	used bool
}

func (sink *refusedSink) Write(chunk []byte) (int, error) { return sink.data.Write(chunk) }

func (sink *refusedSink) ReadFromWithConcurrency(io.Reader, int) (int64, error) {
	sink.used = true
	return 0, errors.New("pipelining refused")
}

func TestUploadCopyPipelinesWithReadFromConcurrency(t *testing.T) {
	worker := NewWorker(func() (Remote, error) { return nil, nil }, nil)
	defer worker.Close()
	payload := bytes.Repeat([]byte("pipelined "), 4096)
	sink := &concurrentSink{}
	written, err := worker.uploadCopy(sink, bytes.NewReader(payload), 0, 0, 0, 0, 0)
	if err != nil || written != int64(len(payload)) || !bytes.Equal(sink.data.Bytes(), payload) {
		t.Fatalf("written=%d sink=%d err=%v", written, sink.data.Len(), err)
	}
	if !sink.used || sink.concurrency != uploadConcurrency {
		t.Fatalf("used=%v concurrency=%d, want %d", sink.used, sink.concurrency, uploadConcurrency)
	}
}

func TestUploadCopyFallsBackToSequentialWrites(t *testing.T) {
	worker := NewWorker(func() (Remote, error) { return nil, nil }, nil)
	defer worker.Close()
	payload := []byte("sequential payload")
	var sink bytes.Buffer
	written, err := worker.uploadCopy(&sink, bytes.NewReader(payload), 0, 0, 0, 0, 0)
	if err != nil || written != int64(len(payload)) || !bytes.Equal(sink.Bytes(), payload) {
		t.Fatalf("written=%d sink=%q err=%v", written, sink.String(), err)
	}
}

func TestUploadCopyRetriesSequentiallyWhenReadFromFailsBeforeReading(t *testing.T) {
	worker := NewWorker(func() (Remote, error) { return nil, nil }, nil)
	defer worker.Close()
	payload := []byte("fallback payload")
	sink := &refusedSink{}
	written, err := worker.uploadCopy(sink, bytes.NewReader(payload), 0, 0, 0, 0, 0)
	if err != nil || written != int64(len(payload)) || !bytes.Equal(sink.data.Bytes(), payload) {
		t.Fatalf("written=%d sink=%q err=%v", written, sink.data.String(), err)
	}
	if !sink.used {
		t.Fatal("the pipelined path was not tried")
	}
}
