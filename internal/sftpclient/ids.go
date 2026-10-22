package sftpclient

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// IDNames maps the numeric owner and group of a file to the names the server's
// account files give them. SFTP carries only the numbers; /etc/passwd and
// /etc/group are the best place to look the names up, and where they say
// nothing (accounts kept elsewhere) the numbers stay.
type IDNames struct {
	Users, Groups map[uint32]string
}

// maxIDFileBytes bounds how much of an account file is read.
const maxIDFileBytes = 1 << 20

// loadIDNames reads the account files through the subsystem, best effort.
func loadIDNames(client Remote) IDNames {
	return IDNames{
		Users:  readIDFile(client, "/etc/passwd"),
		Groups: readIDFile(client, "/etc/group"),
	}
}

func readIDFile(client Remote, path string) map[uint32]string {
	file, err := client.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	return parseIDFile(io.LimitReader(file, maxIDFileBytes))
}

// parseIDFile reads "name:password:id:..." lines. The first name for an id wins,
// which is the one a system lists first.
func parseIDFile(reader io.Reader) map[uint32]string {
	names := map[uint32]string{}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.SplitN(line, ":", 4)
		if len(fields) < 3 || fields[0] == "" || strings.HasPrefix(fields[0], "+") || strings.HasPrefix(fields[0], "-") {
			continue
		}
		id, err := strconv.ParseUint(fields[2], 10, 32)
		if err != nil {
			continue
		}
		if _, taken := names[uint32(id)]; !taken && len(names) < 100_000 {
			names[uint32(id)] = fields[0]
		}
	}
	return names
}
