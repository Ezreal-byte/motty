// Package sessionimport reads the saved sessions of other SSH clients, PuTTY,
// Xshell and MobaXterm, into the host fields Motty keeps. It reads files the
// user chose and never executes, resolves or contacts anything in them.
//
// Only what a connection needs is taken: a name, a folder, an address, a port
// and a user. Passwords are never imported: every one of these clients stores
// them encrypted with a scheme of its own, and a key file path in a session is
// reported rather than followed, since each client has its own key format.
package sessionimport

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Limits keep a damaged or hostile file from using the machine: a session file
// is a few hundred bytes, and a whole export a few megabytes.
const (
	MaxFileSize    = 16 << 20
	MaxEntries     = 5000
	maxArchiveSize = 64 << 20
	maxNameLength  = 200
)

// Protocol is how a session is reached.
type Protocol uint8

const (
	SSH Protocol = iota
	Telnet
	Serial
)

// Entry is one session. A serial one keeps its device in Host and its line in
// Line ("9600 8N1") with Flow "rtscts", "xonxoff" or empty.
type Entry struct {
	Protocol Protocol
	Line     string
	Flow     string

	Name  string
	Group string // folders joined with "/", empty at the top
	Host  string
	Port  int // zero when the file named none
	User  string
	// Source names where the entry came from, for warnings.
	Source string
}

// Result is what a file held.
type Result struct {
	Entries []Entry
	// Skipped counts sessions of a kind that is not read (remote desktop, FTP,
	// and the serial ports of clients whose serial settings are not read).
	Skipped int
	// Keys counts sessions that name a private key file, which was not imported.
	Keys     int
	Warnings []string
}

func (result *Result) warn(format string, args ...any) {
	if len(result.Warnings) < 64 {
		result.Warnings = append(result.Warnings, fmt.Sprintf(format, args...))
	}
}

func (result *Result) add(entry Entry) {
	if len(result.Entries) >= MaxEntries {
		result.warn("%s: more than %d sessions; the rest were not read", entry.Source, MaxEntries)
		return
	}
	entry.Name = clean(entry.Name, maxNameLength)
	entry.Host = clean(entry.Host, 255)
	entry.User = clean(entry.User, 128)
	if entry.Name == "" {
		entry.Name = entry.Host
	}
	if entry.Protocol == Serial {
		if entry.Host == "" || strings.HasPrefix(entry.Host, "-") {
			result.warn("%s: no serial device", entry.Source)
			return
		}
		entry.User, entry.Port = "", 0
		result.Entries = append(result.Entries, entry)
		return
	}
	if entry.Host == "" || strings.ContainsAny(entry.Host, " \t/\\") {
		result.warn("%s: no usable host address", entry.Source)
		return
	}
	if entry.Port < 0 || entry.Port > 65535 {
		entry.Port = 0
	}
	result.Entries = append(result.Entries, entry)
}

func (result *Result) merge(other Result) {
	for _, entry := range other.Entries {
		result.add(entry)
	}
	result.Skipped += other.Skipped
	result.Keys += other.Keys
	for _, warning := range other.Warnings {
		result.warn("%s", warning)
	}
}

// clean makes one printable line of a name: control characters go, the ends are
// trimmed, and a name too long to show is cut at a character.
func clean(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	if len(value) > limit {
		value = strings.ToValidUTF8(value[:limit], "")
	}
	return value
}

// readFile reads at most MaxFileSize bytes and decodes them to UTF-8.
func readFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxFileSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxFileSize {
		return "", fmt.Errorf("%s is too large to be a session file", filepath.Base(path))
	}
	return decode(data), nil
}

// decode turns a file's bytes into text: UTF-16 with a byte-order mark (what
// Windows tools write), or UTF-8 with or without one. Anything that is neither
// is read as UTF-8 with its bad bytes dropped.
func decode(data []byte) string {
	switch {
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}):
		return decodeUTF16(data[2:], false)
	case bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		return decodeUTF16(data[2:], true)
	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		data = data[3:]
	}
	if utf8.Valid(data) {
		return string(data)
	}
	return strings.ToValidUTF8(string(data), "")
}

func decodeUTF16(data []byte, bigEndian bool) string {
	units := make([]uint16, 0, len(data)/2)
	for index := 0; index+1 < len(data); index += 2 {
		if bigEndian {
			units = append(units, uint16(data[index])<<8|uint16(data[index+1]))
		} else {
			units = append(units, uint16(data[index+1])<<8|uint16(data[index]))
		}
	}
	return string(utf16.Decode(units))
}

// ini is a parsed INI file: sections in file order, each with its keys in
// lower case.
type ini struct {
	sections []iniSection
}

type iniSection struct {
	name   string
	values map[string]string
	order  []string // keys as they appeared, lower-cased, for formats whose keys are names
	raw    map[string]string
}

func parseINI(text string) ini {
	var file ini
	current := -1
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || line[0] == ';' || line[0] == '#' {
			continue
		}
		if line[0] == '[' {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				continue
			}
			file.sections = append(file.sections, iniSection{
				name: line[1:end], values: map[string]string{}, raw: map[string]string{},
			})
			current = len(file.sections) - 1
			continue
		}
		if current < 0 {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		section := &file.sections[current]
		if _, seen := section.raw[key]; !seen {
			section.order = append(section.order, key)
		}
		section.raw[key] = value
		section.values[strings.ToLower(key)] = strings.TrimSpace(value)
	}
	return file
}

func (file ini) section(name string) (iniSection, bool) {
	for _, section := range file.sections {
		if strings.EqualFold(section.name, name) {
			return section, true
		}
	}
	return iniSection{}, false
}

// Parse reads each path with the reader for client, and joins the results.
func Parse(client Client, paths []string) (Result, error) {
	var result Result
	if len(paths) == 0 {
		return result, errors.New("no file was chosen")
	}
	var failed []string
	for _, path := range paths {
		var (
			part Result
			err  error
		)
		switch client {
		case PuTTY:
			part, err = ParsePuTTY(path)
		case Xshell:
			part, err = ParseXshell(path)
		case MobaXterm:
			part, err = ParseMobaXterm(path)
		case WindTerm:
			part, err = ParseWindTerm(path)
		default:
			err = fmt.Errorf("unknown client %d", client)
		}
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", filepath.Base(path), err))
			continue
		}
		result.merge(part)
	}
	if len(result.Entries) == 0 && len(failed) > 0 {
		return result, errors.New(strings.Join(failed, "; "))
	}
	for _, failure := range failed {
		result.warn("%s", failure)
	}
	return result, nil
}

// Client says whose files a path holds.
type Client uint8

const (
	PuTTY Client = iota + 1
	Xshell
	MobaXterm
	WindTerm
)

// ---- PuTTY ----

// ParsePuTTY reads a PuTTY session: a file of a Unix PuTTY's sessions folder
// (HostName=... lines), or a Windows registry export (.reg) holding any number.
func ParsePuTTY(path string) (Result, error) {
	text, err := readFile(path)
	if err != nil {
		return Result{}, err
	}
	if strings.HasPrefix(strings.TrimSpace(text), "Windows Registry Editor") {
		return parsePuTTYRegistry(text, filepath.Base(path)), nil
	}
	var result Result
	name, err := url.PathUnescape(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if err != nil {
		name = filepath.Base(path)
	}
	values := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(strings.TrimRight(line, "\r"), "=")
		if ok {
			values[key] = value
		}
	}
	if values["HostName"] == "" {
		return result, errors.New("not a PuTTY session: it has no HostName")
	}
	addPuTTYSession(&result, name, values, filepath.Base(path))
	return result, nil
}

func addPuTTYSession(result *Result, name string, values map[string]string, source string) {
	protocol := strings.ToLower(values["Protocol"])
	switch protocol {
	case "serial":
		addPuTTYSerial(result, name, values, source)
		return
	case "telnet":
		port, _ := strconv.Atoi(values["PortNumber"])
		result.add(Entry{Name: name, Protocol: Telnet, Host: values["HostName"], Port: port, Source: source})
		return
	case "", "ssh":
	default:
		result.Skipped++
		return
	}
	port, _ := strconv.Atoi(values["PortNumber"])
	host, user := values["HostName"], values["UserName"]
	if at := strings.LastIndexByte(host, '@'); at >= 0 && user == "" {
		host, user = host[at+1:], host[:at]
	}
	if values["PublicKeyFile"] != "" {
		result.Keys++
	}
	result.add(Entry{Name: name, Host: host, Port: port, User: user, Source: source})
}

// addPuTTYSerial reads a serial session: SerialLine is the port, SerialSpeed the
// baud rate, SerialParity 0 to 4 for none, odd, even, mark and space,
// SerialStopHalfbits 2 or 4 for one or two stop bits and SerialFlowControl 0 to 3
// for none, XON/XOFF, RTS/CTS and DSR/DTR (which is not available and is read as
// none).
func addPuTTYSerial(result *Result, name string, values map[string]string, source string) {
	speed, _ := strconv.Atoi(values["SerialSpeed"])
	if speed == 0 {
		speed = 9600 // PuTTY's own default
	}
	bits, _ := strconv.Atoi(values["SerialDataBits"])
	if bits == 0 {
		bits = 8
	}
	parity := "NOEMS"
	letter := byte('N')
	if index, err := strconv.Atoi(values["SerialParity"]); err == nil && index >= 0 && index < len(parity) {
		letter = parity[index]
	}
	stop := 1
	if values["SerialStopHalfbits"] == "4" {
		stop = 2
	}
	flow := ""
	switch values["SerialFlowControl"] {
	case "1":
		flow = "xonxoff"
	case "2":
		flow = "rtscts"
	}
	device := values["SerialLine"]
	if device == "" {
		device = "COM1"
	}
	result.add(Entry{Name: name, Protocol: Serial, Host: device, Line: fmt.Sprintf("%d %d%c%d", speed, bits, letter, stop), Flow: flow, Source: source})
}

// parsePuTTYRegistry reads the Sessions keys of an exported
// HKEY_CURRENT_USER\Software\SimonTatham\PuTTY.
func parsePuTTYRegistry(text, source string) Result {
	var (
		result Result
		name   string
		values map[string]string
		inKey  bool
		flush  = func() {
			if inKey && values != nil {
				addPuTTYSession(&result, name, values, source)
			}
			inKey, values = false, nil
		}
		marker = `\software\simontatham\putty\sessions\`
	)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			flush()
			key := line[1 : len(line)-1]
			if at := strings.Index(strings.ToLower(key), marker); at >= 0 {
				raw := key[at+len(marker):]
				if raw == "" || strings.Contains(raw, `\`) {
					continue
				}
				unescaped, err := url.PathUnescape(raw)
				if err != nil {
					unescaped = raw
				}
				name, values, inKey = unescaped, map[string]string{}, true
			}
			continue
		}
		if !inKey || !strings.HasPrefix(line, `"`) {
			continue
		}
		key, value, ok := registryValue(line)
		if ok {
			values[key] = value
		}
	}
	flush()
	return result
}

// registryValue reads one "Name"="text" or "Name"=dword:0000001a line.
func registryValue(line string) (key, value string, ok bool) {
	end := strings.Index(line[1:], `"`)
	if end < 0 {
		return "", "", false
	}
	key = line[1 : 1+end]
	rest := strings.TrimPrefix(line[end+2:], "=")
	switch {
	case strings.HasPrefix(rest, "dword:"):
		number, err := strconv.ParseUint(strings.TrimPrefix(rest, "dword:"), 16, 32)
		if err != nil {
			return "", "", false
		}
		return key, strconv.FormatUint(number, 10), true
	case strings.HasPrefix(rest, `"`) && strings.HasSuffix(rest, `"`) && len(rest) >= 2:
		value = rest[1 : len(rest)-1]
		value = strings.ReplaceAll(value, `\\`, "\x00")
		value = strings.ReplaceAll(value, `\"`, `"`)
		return key, strings.ReplaceAll(value, "\x00", `\`), true
	}
	return "", "", false
}

// ---- Xshell ----

// ParseXshell reads an Xshell session file (.xsh), or an exported archive of
// them (.xts), whose folders become groups.
func ParseXshell(path string) (Result, error) {
	if strings.EqualFold(filepath.Ext(path), ".xts") {
		return parseXshellArchive(path)
	}
	text, err := readFile(path)
	if err != nil {
		return Result{}, err
	}
	var result Result
	if err := parseXshellSession(&result, text, strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), "", filepath.Base(path)); err != nil {
		return result, err
	}
	return result, nil
}

func parseXshellSession(result *Result, text, name, group, source string) error {
	file := parseINI(text)
	connection, ok := file.section("CONNECTION")
	if !ok || connection.values["host"] == "" && connection.values["protocol"] == "" {
		return errors.New("not an Xshell session: it has no [CONNECTION] section")
	}
	telnet := false
	switch protocol := strings.ToUpper(connection.values["protocol"]); protocol {
	case "", "SSH":
	case "TELNET":
		telnet = true
	default:
		result.Skipped++
		return nil
	}
	port, _ := strconv.Atoi(connection.values["port"])
	if telnet {
		result.add(Entry{Name: name, Group: group, Protocol: Telnet, Host: connection.values["host"], Port: port, Source: source})
		return nil
	}
	user := ""
	if auth, ok := file.section("CONNECTION:AUTHENTICATION"); ok {
		user = auth.values["username"]
		if auth.values["userkey"] != "" {
			result.Keys++
		}
	}
	result.add(Entry{Name: name, Group: group, Host: connection.values["host"], Port: port, User: user, Source: source})
	return nil
}

func parseXshellArchive(path string) (Result, error) {
	var result Result
	archive, err := zip.OpenReader(path)
	if err != nil {
		return result, fmt.Errorf("not an Xshell export: %v", err)
	}
	defer archive.Close()
	var total uint64
	for _, member := range archive.File {
		if member.FileInfo().IsDir() || !strings.EqualFold(filepath.Ext(member.Name), ".xsh") {
			continue
		}
		total += member.UncompressedSize64
		if member.UncompressedSize64 > MaxFileSize || total > maxArchiveSize {
			result.warn("%s: too large; the rest of the archive was not read", filepath.Base(path))
			break
		}
		file, err := member.Open()
		if err != nil {
			result.warn("%s: %v", member.Name, err)
			continue
		}
		data, err := io.ReadAll(io.LimitReader(file, MaxFileSize+1))
		file.Close()
		if err != nil || len(data) > MaxFileSize {
			result.warn("%s: unreadable", member.Name)
			continue
		}
		slashed := strings.ReplaceAll(member.Name, `\`, "/")
		group, base := "", slashed
		if at := strings.LastIndexByte(slashed, '/'); at >= 0 {
			group, base = strings.Trim(slashed[:at], "/"), slashed[at+1:]
		}
		if err := parseXshellSession(&result, decode(data), strings.TrimSuffix(base, filepath.Ext(base)), group, member.Name); err != nil {
			result.warn("%s: %v", member.Name, err)
		}
	}
	return result, nil
}

// ---- MobaXterm ----

// ParseMobaXterm reads an exported MobaXterm sessions file (.mxtsessions), or the
// [Bookmarks] sections of MobaXterm.ini. A session is a line
// Name=#icon#type%host%port%user%...; type 0 is SSH. Folders are the SubRep of
// each [Bookmarks] section.
func ParseMobaXterm(path string) (Result, error) {
	text, err := readFile(path)
	if err != nil {
		return Result{}, err
	}
	var result Result
	file := parseINI(text)
	found := false
	for _, section := range file.sections {
		if !strings.HasPrefix(strings.ToLower(section.name), "bookmarks") {
			continue
		}
		found = true
		group := strings.Trim(strings.ReplaceAll(section.values["subrep"], `\`, "/"), "/")
		for _, key := range section.order {
			if strings.EqualFold(key, "SubRep") || strings.EqualFold(key, "ImgNum") {
				continue
			}
			parseMobaSession(&result, key, strings.TrimSpace(section.raw[key]), group, filepath.Base(path))
		}
	}
	if !found {
		return result, errors.New("not a MobaXterm sessions file: it has no [Bookmarks] section")
	}
	return result, nil
}

func parseMobaSession(result *Result, name, value, group, source string) {
	if !strings.HasPrefix(value, "#") {
		return
	}
	rest := value[1:]
	hash := strings.IndexByte(rest, '#')
	if hash < 0 {
		return
	}
	fields := strings.Split(rest[hash+1:], "%")
	if len(fields) < 4 {
		return
	}
	port, _ := strconv.Atoi(fields[2])
	switch fields[0] {
	case "0":
		result.add(Entry{Name: name, Group: group, Host: fields[1], Port: port, User: fields[3], Source: source})
	case "1":
		result.add(Entry{Name: name, Group: group, Protocol: Telnet, Host: fields[1], Port: port, Source: source})
	default:
		result.Skipped++
	}
}

// ---- WindTerm ----

// windTermSession is one entry of WindTerm's user.sessions, a JSON array it
// keeps under profiles/<profile>/terminal/. A saved login (session.autoLogin) is
// encrypted and is not read.
type windTermSession struct {
	Label    string `json:"session.label"`
	Group    string `json:"session.group"`
	Target   string `json:"session.target"`
	Port     int    `json:"session.port"`
	Protocol string `json:"session.protocol"`
	User     string `json:"session.user"`
	Identity string `json:"session.identityFile"`
}

// ParseWindTerm reads a WindTerm user.sessions file.
func ParseWindTerm(path string) (Result, error) {
	text, err := readFile(path)
	if err != nil {
		return Result{}, err
	}
	var sessions []windTermSession
	if err := json.Unmarshal([]byte(text), &sessions); err != nil {
		return Result{}, errors.New("not a WindTerm sessions file: it is not a JSON list of sessions")
	}
	var result Result
	source := filepath.Base(path)
	for _, session := range sessions {
		if session.Target == "" && session.Protocol == "" && session.Label == "" {
			continue
		}
		kind := SSH
		switch strings.ToUpper(session.Protocol) {
		case "", "SSH":
		case "TELNET":
			kind = Telnet
		default:
			result.Skipped++
			continue
		}
		if session.Identity != "" {
			result.Keys++
		}
		host, user := session.Target, session.User
		if at := strings.LastIndexByte(host, '@'); at >= 0 && user == "" {
			host, user = host[at+1:], host[:at]
		}
		if kind == Telnet {
			user = ""
		}
		result.add(Entry{
			Name: session.Label, Group: strings.Trim(strings.ReplaceAll(session.Group, `\`, "/"), "/"),
			Protocol: kind, Host: host, Port: session.Port, User: user, Source: source,
		})
	}
	return result, nil
}
