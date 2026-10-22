package scpclient

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// SCP has no directory listing, so a listing is whatever `ls` prints. It is
// approximate: names that contain a newline cannot be told apart, times are in
// the server's zone without seconds where `ls` drops them, and sizes of special
// files are zero.

// Item is one line of a listing.
type Item struct {
	Name       string
	Size       int64
	Mode       os.FileMode
	ModTime    time.Time
	UID, GID   uint32
	LinkTarget string
}

// maxListingBytes bounds the output of one listing.
const maxListingBytes = 16 << 20

// listCommand tries GNU ls with epoch times, then any ls.
func listCommand(flags, quoted string) string {
	return "LC_ALL=C ls " + flags + " --time-style=+%s -- " + quoted + " 2>/dev/null || LC_ALL=C ls " + flags + " -- " + quoted
}

// List reads a directory.
func (c *Client) List(directory string) ([]Item, error) {
	command, err := Command(listCommand("-lAn", "{}"), directory, directory)
	if err != nil {
		return nil, err
	}
	out, err := c.Run(command, maxListingBytes)
	if err != nil {
		return nil, mapMissing(err)
	}
	return ParseListing(out, time.Now())
}

// Stat describes one path. With follow a symbolic link is resolved.
func (c *Client) Stat(name string, follow bool) (Item, error) {
	flags := "-lAdn"
	if follow {
		flags = "-lAdnL"
	}
	command, err := Command(listCommand(flags, "{}"), name, name)
	if err != nil {
		return Item{}, err
	}
	out, err := c.Run(command, 64<<10)
	if err != nil {
		return Item{}, mapMissing(err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if item, ok := parseListingLine(strings.TrimRight(line, "\r"), time.Now(), true); ok {
			return item, nil
		}
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return Item{}, fmt.Errorf("%s: %w", name, os.ErrNotExist)
	}
	return Item{}, fmt.Errorf("scp: cannot read the entry of %q", name)
}

func mapMissing(err error) error {
	if re, ok := err.(*RemoteError); ok && re.Is(os.ErrNotExist) {
		return fmt.Errorf("%s: %w", re.Message, os.ErrNotExist)
	}
	return err
}

// ParseListing reads `ls -lAn` output, defensively: lines it cannot read are
// skipped, and names that could escape a directory are dropped.
func ParseListing(out []byte, now time.Time) ([]Item, error) {
	var items []Item
	for _, raw := range bytes.Split(out, []byte{'\n'}) {
		line := strings.TrimRight(string(raw), "\r")
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		item, ok := parseListingLine(line, now, false)
		if !ok || len(items) >= 200_000 {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func nextField(s string) (field, rest string) {
	s = strings.TrimLeft(s, " \t")
	end := strings.IndexAny(s, " \t")
	if end < 0 {
		return s, ""
	}
	return s[:end], s[end+1:]
}

var months = map[string]time.Month{
	"Jan": 1, "Feb": 2, "Mar": 3, "Apr": 4, "May": 5, "Jun": 6,
	"Jul": 7, "Aug": 8, "Sep": 9, "Oct": 10, "Nov": 11, "Dec": 12,
}

func parseListingLine(line string, now time.Time, named bool) (Item, bool) {
	modeText, rest := nextField(line)
	if len(modeText) < 10 {
		return Item{}, false
	}
	mode, ok := parseModeText(modeText)
	if !ok {
		return Item{}, false
	}
	var fields [4]string // links, uid, gid, size
	for index := range fields {
		fields[index], rest = nextField(rest)
	}
	uid, uidErr := strconv.ParseUint(fields[1], 10, 32)
	gid, gidErr := strconv.ParseUint(fields[2], 10, 32)
	if fields[0] == "" || uidErr != nil || gidErr != nil {
		return Item{}, false
	}
	var size int64
	if strings.HasSuffix(fields[3], ",") { // a device: "major, minor"
		_, rest = nextField(rest)
	} else if parsed, err := strconv.ParseInt(fields[3], 10, 64); err == nil && parsed >= 0 {
		size = parsed
	} else {
		return Item{}, false
	}
	var modTime time.Time
	stamp, after := nextField(rest)
	if seconds, err := strconv.ParseInt(stamp, 10, 64); err == nil {
		modTime, rest = time.Unix(seconds, 0), after
	} else if month, known := months[stamp]; known {
		day, afterDay := nextField(after)
		clock, afterClock := nextField(afterDay)
		dayNumber, dayErr := strconv.Atoi(day)
		if dayErr != nil {
			return Item{}, false
		}
		rest = afterClock
		if hour, minute, isClock := strings.Cut(clock, ":"); isClock {
			h, hErr := strconv.Atoi(hour)
			m, mErr := strconv.Atoi(minute)
			if hErr != nil || mErr != nil {
				return Item{}, false
			}
			modTime = time.Date(now.Year(), month, dayNumber, h, m, 0, 0, time.UTC)
			if modTime.After(now.Add(48 * time.Hour)) {
				modTime = modTime.AddDate(-1, 0, 0)
			}
		} else if year, err := strconv.Atoi(clock); err == nil {
			modTime = time.Date(year, month, dayNumber, 0, 0, 0, 0, time.UTC)
		} else {
			return Item{}, false
		}
	} else {
		return Item{}, false
	}
	name, target := rest, ""
	if mode&os.ModeSymlink != 0 {
		if before, after, found := strings.Cut(name, " -> "); found {
			name, target = before, after
		}
	}
	if named {
		// `ls -d` echoes the path it was given.
		name = name[strings.LastIndex(name, "/")+1:]
	}
	if name == "" || (!named && checkName(name) != nil) || strings.ContainsRune(name, 0) {
		return Item{}, false
	}
	return Item{Name: name, Size: size, Mode: mode, ModTime: modTime, UID: uint32(uid), GID: uint32(gid), LinkTarget: target}, true
}

// parseModeText reads the "drwxr-xr-x" column.
func parseModeText(text string) (os.FileMode, bool) {
	var mode os.FileMode
	switch text[0] {
	case '-':
	case 'd':
		mode |= os.ModeDir
	case 'l':
		mode |= os.ModeSymlink
	case 'c':
		mode |= os.ModeDevice | os.ModeCharDevice
	case 'b':
		mode |= os.ModeDevice
	case 'p':
		mode |= os.ModeNamedPipe
	case 's':
		mode |= os.ModeSocket
	default:
		return 0, false
	}
	bits := text[1:10]
	for index, r := range bits {
		shift := uint(8 - index)
		switch r {
		case 'r', 'w', 'x':
			mode |= 1 << shift
		case 's', 't':
			mode |= 1 << shift
			mode |= special(index)
		case 'S', 'T':
			mode |= special(index)
		case '-':
		default:
			return 0, false
		}
	}
	return mode, true
}

func special(index int) os.FileMode {
	switch index {
	case 2:
		return os.ModeSetuid
	case 5:
		return os.ModeSetgid
	}
	return os.ModeSticky
}
