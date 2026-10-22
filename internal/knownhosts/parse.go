package knownhosts

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

type documentLine struct {
	start    int
	end      int
	entry    Entry
	hasEntry bool
	remove   bool
}

func parseDocument(data []byte) ([]documentLine, []Entry) {
	var lines []documentLine
	var entries []Entry
	occurrences := make(map[[sha256.Size]byte]int)

	for start, lineNumber := 0, 1; start < len(data); lineNumber++ {
		newline := bytes.IndexByte(data[start:], '\n')
		end := len(data)
		bodyEnd := end
		if newline >= 0 {
			bodyEnd = start + newline
			end = bodyEnd + 1
		}
		if bodyEnd > start && data[bodyEnd-1] == '\r' {
			bodyEnd--
		}

		line := documentLine{start: start, end: end}
		body := data[start:bodyEnd]
		if entry, ok := parseEntry(body); ok {
			digest := sha256.Sum256(body)
			occurrences[digest]++
			entry.ID = makeID(digest, occurrences[digest])
			entry.Line = lineNumber
			line.entry = entry
			line.hasEntry = true
			entries = append(entries, entry)
		}
		lines = append(lines, line)
		start = end
	}
	return lines, entries
}

func parseEntry(line []byte) (Entry, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] == '#' {
		return Entry{}, false
	}

	first, rest := nextToken(line)
	marker := ""
	if len(first) > 0 && first[0] == '@' {
		marker = string(first)
		if marker != "@cert-authority" && marker != "@revoked" {
			return Entry{}, false
		}
		first, rest = nextToken(rest)
	}
	hosts := first
	keyType, rest := nextToken(rest)
	encodedKey, comment := nextToken(rest)
	if len(hosts) == 0 || len(keyType) == 0 || len(encodedKey) == 0 {
		return Entry{}, false
	}

	keyBytes, err := base64.StdEncoding.DecodeString(string(encodedKey))
	if err != nil {
		return Entry{}, false
	}
	key, err := ssh.ParsePublicKey(keyBytes)
	if err != nil || key.Type() != string(keyType) {
		return Entry{}, false
	}

	patterns := strings.Split(string(hosts), ",")
	hashed := false
	for _, pattern := range patterns {
		if pattern == "" {
			return Entry{}, false
		}
		hashed = hashed || strings.HasPrefix(pattern, "|")
	}
	return Entry{
		Marker:       marker,
		HostPatterns: patterns,
		KeyType:      key.Type(),
		Fingerprint:  ssh.FingerprintSHA256(key),
		Comment:      string(bytes.TrimSpace(comment)),
		Hashed:       hashed,
	}, true
}

func nextToken(data []byte) ([]byte, []byte) {
	data = bytes.TrimLeft(data, " \t")
	if len(data) == 0 {
		return nil, nil
	}
	if index := bytes.IndexAny(data, " \t"); index >= 0 {
		return data[:index], data[index:]
	}
	return data, nil
}

func makeID(digest [sha256.Size]byte, occurrence int) ID {
	encoded := make([]byte, hex.EncodedLen(len(digest)))
	hex.Encode(encoded, digest[:])
	encoded = append(encoded, '-')
	encoded = strconv.AppendInt(encoded, int64(occurrence), 10)
	return ID(encoded)
}
