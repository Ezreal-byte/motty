package scpclient

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"time"
)

// Handler receives what a download delivers. Names are slash-separated paths
// relative to the downloaded root, built only from validated components.
type Handler interface {
	// Dir is called before the entries of a directory. The root directory is ".".
	Dir(relative string, mode fs.FileMode, mtime time.Time) error
	// File returns where the size bytes of a file go. The root file is ".".
	File(relative string, mode fs.FileMode, mtime time.Time, size int64) (FileWriter, error)
}

// FileWriter takes the bytes of one downloaded file. Exactly one of Commit and
// Abort ends it: Commit once all announced bytes arrived and the server
// confirmed them, Abort on any failure.
type FileWriter interface {
	io.Writer
	Commit() error
	Abort()
}

// Stats counts what a transfer moved.
type Stats struct {
	Files int
	Bytes int64
}

// Download fetches remote (a file, or with recursive a directory tree) in
// source mode and hands it to handler.
func (c *Client) Download(remote string, recursive bool, handler Handler) (Stats, error) {
	quoted, err := Quote(remote)
	if err != nil {
		return Stats{}, err
	}
	command := "scp -f -p -- " + quoted
	if recursive {
		command = "scp -f -r -p -- " + quoted
	}
	ch, err := c.start(command)
	if err != nil {
		return Stats{}, err
	}
	stats, err := c.receive(newPeer(ch), recursive, handler)
	return stats, finish(ch, err)
}

func (c *Client) receive(p *peer, recursive bool, handler Handler) (stats Stats, err error) {
	if err := p.ack(); err != nil {
		return stats, err
	}
	var (
		stack    []string // names of the open directories
		entries  int
		rootDone bool
		mtime    time.Time
	)
	for {
		kind, err := p.reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(stack) > 0 {
					return stats, io.ErrUnexpectedEOF
				}
				if !rootDone {
					return stats, fmt.Errorf("%w: the server sent nothing", ErrProtocol)
				}
				return stats, nil
			}
			return stats, err
		}
		switch kind {
		case 1, 2:
			return stats, p.remoteError(kind == 2)
		case 'C', 'D', 'E', 'T':
		default:
			// A shell's own complaint arrives as plain text.
			rest, _ := p.readLine()
			return stats, &RemoteError{Fatal: true, Message: sanitize(string(append([]byte{kind}, rest...)))}
		}
		line, err := p.readLine()
		if err != nil {
			return stats, err
		}
		message, err := parseControl(append([]byte{kind}, line...))
		if err != nil {
			return stats, err
		}
		switch message.kind {
		case 'T':
			mtime = message.mtime
			if err := p.ack(); err != nil {
				return stats, err
			}
		case 'E':
			if len(stack) == 0 {
				return stats, fmt.Errorf("%w: E without a directory", ErrProtocol)
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				rootDone = true
			}
			if err := p.ack(); err != nil {
				return stats, err
			}
		case 'D', 'C':
			if rootDone {
				return stats, fmt.Errorf("%w: more than the one requested entry", ErrProtocol)
			}
			if message.kind == 'D' && !recursive {
				return stats, fmt.Errorf("%w: directory sent for a file request", ErrProtocol)
			}
			entries++
			if entries > c.limits.MaxEntries {
				return stats, fmt.Errorf("%w: more than %d entries", ErrTooLarge, c.limits.MaxEntries)
			}
			modified := mtime
			mtime = time.Time{}
			// The root is "."; everything below it is named from the root's own level.
			rel := "."
			if len(stack) > 0 {
				rel = strings.Join(append(append([]string(nil), stack[1:]...), message.name), "/")
			}
			if message.kind == 'D' {
				if len(stack) >= c.limits.MaxDepth {
					return stats, fmt.Errorf("%w: deeper than %d directories", ErrTooLarge, c.limits.MaxDepth)
				}
				if err := handler.Dir(rel, message.mode, modified); err != nil {
					return stats, err
				}
				stack = append(stack, message.name)
				if err := p.ack(); err != nil {
					return stats, err
				}
				continue
			}
			if message.size > c.limits.MaxFileSize || stats.Bytes+message.size > c.limits.MaxTotalSize {
				return stats, fmt.Errorf("%w: %d bytes announced for %q", ErrTooLarge, message.size, message.name)
			}
			if err := c.receiveFile(p, handler, rel, message, modified); err != nil {
				return stats, err
			}
			stats.Files++
			stats.Bytes += message.size
			if len(stack) == 0 {
				rootDone = true
			}
		}
	}
}

func (c *Client) receiveFile(p *peer, handler Handler, relative string, message control, mtime time.Time) error {
	writer, err := handler.File(relative, message.mode, mtime, message.size)
	if err != nil {
		return err
	}
	if err := c.receiveBody(p, writer, message); err != nil {
		writer.Abort()
		return err
	}
	if err := writer.Commit(); err != nil {
		return err
	}
	return p.ack()
}

func (c *Client) receiveBody(p *peer, writer io.Writer, message control) error {
	if err := p.ack(); err != nil {
		return err
	}
	buffer := copyBuffers.Get().(*[64 * 1024]byte)
	defer copyBuffers.Put(buffer)
	remaining := message.size
	for remaining > 0 {
		chunk := buffer[:min(int64(len(buffer)), remaining)]
		read, readErr := p.reader.Read(chunk)
		if read > 0 {
			if _, err := writer.Write(chunk[:read]); err != nil {
				return err
			}
			remaining -= int64(read)
		}
		if readErr != nil {
			if remaining == 0 {
				break
			}
			if errors.Is(readErr, io.EOF) {
				return fmt.Errorf("%w: %q ends %d bytes short", io.ErrUnexpectedEOF, message.name, remaining)
			}
			return readErr
		}
	}
	return p.readStatus()
}
