package xymodem

import (
	"fmt"
	"io"

	"motty/internal/zmodem"
)

type receiverState uint8

const (
	rWaitBlock     receiverState = iota // between blocks: a header byte is next
	rInBlock                            // collecting the rest of a block
	rWaitSecondEOT                      // YMODEM: the first EOT was answered with NAK
	rDone
)

// Receiver takes files from a sender. For XMODEM it writes one file to the
// target given, whatever its name; for YMODEM it opens a file for each name the
// sender announces.
type Receiver struct {
	base
	mode   Mode
	target zmodem.Target
	name   string // the name asked for XMODEM, which the stream does not carry

	state  receiverState
	useCRC bool
	// started says a block has arrived, so 'C' is no longer sent.
	started bool
	// block is the one being collected.
	size   int
	header byte
	buf    []byte
	last   byte // the number of the last block taken
	// file being written and what is held back until the next block says the
	// one before was not the last.
	file    io.WriteCloser
	info    zmodem.FileInfo
	written int64
	held    []byte
	files   int
	// info0 says the next block is YMODEM's block 0.
	expectInfo bool
	asked      int // 'C' or NAK sent while waiting for the first block
}

// NewReceiver starts a receiver. name is what XMODEM saves as; YMODEM ignores it.
// Its first output is the byte that asks the sender to begin.
func NewReceiver(mode Mode, name string, target zmodem.Target) *Receiver {
	r := &Receiver{mode: mode, target: target, name: name, useCRC: true, expectInfo: mode == YMODEM}
	r.last = 0
	r.ask()
	return r
}

func (r *Receiver) Progress() zmodem.Progress {
	progress := zmodem.Progress{Files: r.files}
	if r.file != nil {
		progress.Name, progress.Size, progress.Done = r.info.Name, r.info.Size, r.written
	}
	return progress
}

// ask sends the byte that invites the sender to start or go on.
func (r *Receiver) ask() {
	if r.useCRC {
		r.out = append(r.out, crc)
	} else {
		r.out = append(r.out, nak)
	}
}

func (r *Receiver) Cancel() {
	if r.finished {
		return
	}
	r.discard()
	r.abort(ErrCancelled)
}

func (r *Receiver) Close() { r.closeFile() }

func (r *Receiver) closeFile() {
	if r.file != nil {
		r.file.Close()
		r.file = nil
	}
}

// discard closes the file as unfinished: the target sees a short file.
func (r *Receiver) discard() { r.closeFile() }

// Timeout is called when nothing has arrived for a while.
func (r *Receiver) Timeout() {
	if r.finished {
		return
	}
	r.quiet++
	if r.quiet > maxRetries {
		r.discard()
		r.abort(ErrTimeout)
		return
	}
	switch {
	case !r.started:
		// Nothing yet: ask again, and after a few tries for plain XMODEM give up
		// on CRC for a sender that only does the checksum.
		r.asked++
		if r.mode != YMODEM && r.useCRC && r.asked >= 3 {
			r.useCRC = false
		}
		r.ask()
	case r.state == rInBlock:
		r.state, r.buf = rWaitBlock, r.buf[:0]
		r.out = append(r.out, nak)
	case r.state == rWaitSecondEOT:
		r.out = append(r.out, nak)
	default:
		r.out = append(r.out, nak)
	}
}

func (r *Receiver) Feed(data []byte) {
	for index, value := range data {
		if r.finished {
			r.leftover = append(r.leftover, data[index:]...)
			return
		}
		r.quiet = 0
		// CAN bytes inside a block are data; only between blocks do two in a row
		// cancel.
		if r.state != rInBlock && r.cancelled(value) {
			r.discard()
			r.fail(ErrPeerCancelled)
			continue
		} else if r.state == rInBlock {
			r.cans = 0
		}
		switch r.state {
		case rWaitBlock, rWaitSecondEOT:
			r.headerByte(value)
		case rInBlock:
			r.buf = append(r.buf, value)
			if len(r.buf) == r.blockLength() {
				r.endBlock()
			}
		}
	}
}

// blockLength is the bytes that follow the header byte of the block being read.
func (r *Receiver) blockLength() int {
	check := 1
	if r.useCRC {
		check = 2
	}
	return 2 + r.size + check
}

func (r *Receiver) headerByte(value byte) {
	switch value {
	case soh, stx:
		r.size, r.header = 128, value
		if value == stx {
			r.size = 1024
		}
		r.state, r.buf = rInBlock, r.buf[:0]
		r.started = true
	case eot:
		r.endOfFile()
	}
	// Anything else is line noise between blocks.
}

// endOfFile answers an EOT.
func (r *Receiver) endOfFile() {
	if r.mode == YMODEM && r.state != rWaitSecondEOT {
		// The first EOT is refused, to be sure it was not noise.
		r.out = append(r.out, nak)
		r.state = rWaitSecondEOT
		return
	}
	r.flushHeld(true)
	if r.file == nil && r.mode != YMODEM {
		// An EOT before any block: an empty file.
		if r.openFile(zmodem.FileInfo{Name: plainName(r.name), Size: 0}) != nil {
			return
		}
	}
	r.completeFile()
	r.out = append(r.out, ack)
	if r.mode == YMODEM {
		// Ready for the next file's block 0.
		r.state, r.expectInfo, r.last, r.started = rWaitBlock, true, 0, false
		r.ask()
		return
	}
	r.state = rDone
	r.finished = true
}

// completeFile closes the file that has just ended.
func (r *Receiver) completeFile() {
	if r.file != nil {
		err := r.file.Close()
		r.file = nil
		r.files++
		if err != nil {
			r.abort(fmt.Errorf("xymodem: finishing %s: %w", r.info.Name, err))
		}
	}
}

func (r *Receiver) openFile(info zmodem.FileInfo) error {
	file, err := r.target.Open(info)
	switch {
	case err != nil:
		r.abort(err)
		return err
	case file == nil:
		r.abort(fmt.Errorf("xymodem: no place to put %s", info.Name))
		return io.ErrClosedPipe
	}
	r.file, r.info, r.written, r.held = file, info, 0, nil
	return nil
}

// endBlock checks and takes a complete block.
func (r *Receiver) endBlock() {
	r.state = rWaitBlock
	body := r.buf
	number, complement := body[0], body[1]
	data := body[2 : 2+r.size]
	good := number^complement == 0xFF
	if good {
		if r.useCRC {
			got := uint16(body[2+r.size])<<8 | uint16(body[3+r.size])
			good = crc16(data) == got
		} else {
			good = checksum(data) == body[2+r.size]
		}
	}
	if !good {
		r.damaged()
		return
	}
	expected := r.last + 1
	switch {
	case number == expected || r.expectInfo && number == 0:
		r.retries = 0
		r.takeBlock(number, data)
	case number == r.last && !r.expectInfo:
		// The sender did not hear the acknowledgement and sent it again.
		r.out = append(r.out, ack)
	default:
		r.abort(ErrOutOfStep)
	}
}

func (r *Receiver) damaged() {
	r.retries++
	if r.retries > maxRetries {
		r.discard()
		r.abort(ErrTooManyErrors)
		return
	}
	r.out = append(r.out, nak)
}

func (r *Receiver) takeBlock(number byte, data []byte) {
	if r.expectInfo {
		r.takeInfo(data)
		return
	}
	r.last = number
	if r.file == nil && r.mode != YMODEM {
		if r.openFile(zmodem.FileInfo{Name: plainName(r.name), Size: -1}) != nil {
			return
		}
	}
	// The block before this one was not the last after all.
	r.flushHeld(false)
	r.held = append(r.held[:0], data...)
	if r.mode == YMODEM && r.info.Size >= 0 {
		// The size says how much of the last block is the file, so nothing need be
		// guessed: write at once, cut at the size.
		r.flushHeld(true)
	}
	r.out = append(r.out, ack)
}

// takeInfo handles YMODEM's block 0.
func (r *Receiver) takeInfo(data []byte) {
	if len(data) == 0 || data[0] == 0 {
		// An empty name ends the batch.
		r.out = append(r.out, ack)
		r.finished, r.state = true, rDone
		return
	}
	if len(data) > maxFileInfo {
		data = data[:maxFileInfo]
	}
	info := parseFileInfo(data)
	if r.openFile(info) != nil {
		return
	}
	r.expectInfo, r.last = false, 0
	r.out = append(r.out, ack)
	r.ask() // and then the data
}

// flushHeld writes the block held back. At the end of an XMODEM file it removes
// the SUB padding the sender added; at the end of a YMODEM file it cuts at the
// announced size.
func (r *Receiver) flushHeld(final bool) {
	if len(r.held) == 0 || r.file == nil {
		return
	}
	data := r.held
	r.held = r.held[:0]
	if r.mode == YMODEM && r.info.Size >= 0 {
		if remaining := r.info.Size - r.written; int64(len(data)) > remaining {
			data = data[:max(remaining, 0)]
		}
	} else if final && r.mode != YMODEM {
		for len(data) > 0 && data[len(data)-1] == sub {
			data = data[:len(data)-1]
		}
	}
	if len(data) == 0 {
		return
	}
	if _, err := r.file.Write(data); err != nil {
		r.closeFile()
		r.abort(fmt.Errorf("xymodem: writing %s: %w", r.info.Name, err))
		return
	}
	r.written += int64(len(data))
}
