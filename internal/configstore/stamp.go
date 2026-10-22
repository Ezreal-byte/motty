package configstore

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Stamp orders the versions of one record across devices. It is a hybrid
// logical clock reading, "<wall ms>.<counter>.<device>" with the first two
// fields in fixed-width hex, so comparing two stamps as strings compares them
// as clocks: the later wall time wins, the counter breaks a tie inside one
// millisecond, and the device id breaks a tie between devices.
type Stamp string

const (
	wallDigits    = 12
	counterDigits = 4
	deviceDigits  = 8
	stampLength   = wallDigits + 1 + counterDigits + 1 + deviceDigits
	maxCounter    = 1<<(4*counterDigits) - 1
)

// CutoffStamp is a stamp that sorts before every one issued at or after t, for
// telling what is older than t.
func CutoffStamp(t time.Time) Stamp { return Stamp(fmt.Sprintf("%0*x", wallDigits, t.UnixMilli())) }

// MaxFuture is how far ahead of the local clock a received stamp may be. A
// stamp further ahead would outrank every later edit made here, so it is
// refused instead of merged.
const MaxFuture = 7 * 24 * time.Hour

func makeStamp(wall int64, counter uint32, device string) Stamp {
	return Stamp(fmt.Sprintf("%0*x.%0*x.%s", wallDigits, wall, counterDigits, counter, device))
}

// valid reports whether the stamp has the form makeStamp gives.
func (stamp Stamp) valid() bool {
	text := string(stamp)
	if len(text) != stampLength || text[wallDigits] != '.' || text[wallDigits+1+counterDigits] != '.' {
		return false
	}
	return isHex(text[:wallDigits]) && isHex(text[wallDigits+1:wallDigits+1+counterDigits]) && isHex(text[stampLength-deviceDigits:])
}

func isHex(text string) bool {
	for _, char := range text {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}

// Time is the wall-clock part of the stamp, or the zero time for one that is
// not valid.
func (stamp Stamp) Time() time.Time {
	if !stamp.valid() {
		return time.Time{}
	}
	wall, err := strconv.ParseInt(string(stamp[:wallDigits]), 16, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(wall)
}

func (stamp Stamp) parts() (wall int64, counter uint32) {
	wall, _ = strconv.ParseInt(string(stamp[:wallDigits]), 16, 64)
	value, _ := strconv.ParseUint(string(stamp[wallDigits+1:wallDigits+1+counterDigits]), 16, 32)
	return wall, uint32(value)
}

// clock issues stamps that never go backwards, even when the system clock does
// or when records from a device whose clock is ahead have been seen.
type clock struct {
	device  string
	wall    int64
	counter uint32
}

// next issues a stamp later than every one issued or observed before.
func (c *clock) next(now time.Time) Stamp {
	wall := now.UnixMilli()
	switch {
	case wall > c.wall:
		c.wall, c.counter = wall, 0
	case c.counter < maxCounter:
		c.counter++
	default:
		c.wall, c.counter = c.wall+1, 0
	}
	return makeStamp(c.wall, c.counter, c.device)
}

// observe moves the clock past a stamp that came from elsewhere.
func (c *clock) observe(stamp Stamp) {
	wall, counter := stamp.parts()
	if wall > c.wall || wall == c.wall && counter > c.counter {
		c.wall, c.counter = wall, counter
	}
}

func (c *clock) String() string {
	return strings.TrimSuffix(string(makeStamp(c.wall, c.counter, c.device)), "."+c.device)
}

func (c *clock) load(text string) {
	if len(text) != wallDigits+1+counterDigits {
		return
	}
	stamp := Stamp(text + "." + c.device)
	if stamp.valid() {
		c.wall, c.counter = stamp.parts()
	}
}
