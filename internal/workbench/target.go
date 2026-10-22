package workbench

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

// errBadTarget means the typed text does not name something to connect to.
var errBadTarget = errors.New("not a connection target")

// target is a destination read from what the user typed.
type target struct {
	user string
	host string
	port uint16 // defaultPort when none was given
}

// parseTarget reads a connection target the way people write one:
//
//	host
//	user@host
//	user@host:2222
//	user@[::1]:2222
//	ssh user@host
//	ssh -p 2222 user@host
//	ssh -l user host
//	ssh://user@host:2222/
//
// Anything else, including other ssh options, is rejected rather than guessed
// at: connecting somewhere the user did not mean is worse than saying no.
func parseTarget(input string) (target, error) {
	fields := strings.Fields(input)
	if len(fields) == 1 && len(fields[0]) > len("ssh://") && strings.EqualFold(fields[0][:len("ssh://")], "ssh://") {
		// A link has no options and no path: ssh://user@host:port, with a slash
		// after it at most.
		fields[0] = strings.TrimSuffix(fields[0][len("ssh://"):], "/")
	} else if len(fields) > 0 && fields[0] == "ssh" {
		fields = fields[1:]
	}
	var destination, port, login string
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		switch {
		case field == "-p":
			if index+1 >= len(fields) {
				return target{}, errBadTarget
			}
			index++
			port = fields[index]
		case strings.HasPrefix(field, "-p") && len(field) > 2:
			port = field[2:]
		case field == "-l":
			if index+1 >= len(fields) {
				return target{}, errBadTarget
			}
			index++
			login = fields[index]
		case strings.HasPrefix(field, "-l") && len(field) > 2:
			login = field[2:]
		case strings.HasPrefix(field, "-"):
			return target{}, errBadTarget
		case destination != "":
			return target{}, errBadTarget
		default:
			destination = field
		}
	}
	if destination == "" {
		return target{}, errBadTarget
	}

	result := target{port: defaultPort}
	if at := strings.LastIndex(destination, "@"); at >= 0 {
		result.user, destination = destination[:at], destination[at+1:]
		if result.user == "" {
			return target{}, errBadTarget
		}
	}
	host, embedded, err := splitHostPort(destination)
	if err != nil || !validHost(host) {
		return target{}, errBadTarget
	}
	result.host = host
	if login != "" && result.user == "" {
		result.user = login // user@host wins over -l, as ssh does
	}
	if embedded != "" && port == "" {
		port = embedded
	}
	if port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return target{}, errBadTarget
		}
		result.port = uint16(value)
	}
	return result, nil
}

// splitHostPort separates "host", "host:port", "[v6]" and "[v6]:port"; a bare
// IPv6 address (several colons, no brackets) is all host.
func splitHostPort(destination string) (host, port string, err error) {
	if strings.HasPrefix(destination, "[") {
		if !strings.Contains(destination, "]:") {
			if !strings.HasSuffix(destination, "]") {
				return "", "", errBadTarget
			}
			return destination[1 : len(destination)-1], "", nil
		}
		return net.SplitHostPort(destination)
	}
	switch strings.Count(destination, ":") {
	case 0:
		return destination, "", nil
	case 1:
		return net.SplitHostPort(destination)
	default:
		return destination, "", nil
	}
}

// validZone accepts an interface name or numeric scope of an IPv6 zone, which
// is what a link-local address is scoped with.
func validZone(zone string) bool {
	if zone == "" || len(zone) > 16 {
		return false
	}
	for _, r := range zone {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

// validHost accepts DNS-style names, IP literals and zone-scoped IPv6
// literals such as fe80::1%en0.
func validHost(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	if base, zone, found := strings.Cut(host, "%"); found && zone != "" {
		return strings.Contains(base, ":") && net.ParseIP(base) != nil && validZone(zone)
	}
	if net.ParseIP(host) != nil {
		return true
	}
	if strings.Contains(host, ":") {
		return false
	}
	for _, r := range host {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
		default:
			return false
		}
	}
	return !strings.HasPrefix(host, "-")
}

// connection is a destination read from what the user typed, with how to reach
// it: ssh by default, or telnet when the text says so.
type connection struct {
	target
	protocol hostProtocol
}

// parseConnection is parseTarget that also understands Telnet:
//
//	telnet host
//	telnet host 2323
//	telnet://host:2323/
func parseConnection(input string) (connection, error) {
	fields := strings.Fields(input)
	rest := ""
	switch {
	case len(fields) == 1 && len(fields[0]) > len("telnet://") && strings.EqualFold(fields[0][:len("telnet://")], "telnet://"):
		rest = strings.TrimSuffix(fields[0][len("telnet://"):], "/")
	case len(fields) >= 2 && len(fields) <= 3 && fields[0] == "telnet":
		rest = fields[1]
		if len(fields) == 3 {
			rest += ":" + fields[2]
		}
	case len(fields) > 0 && fields[0] == "telnet":
		return connection{}, errBadTarget // the word alone, or with too many arguments
	default:
		parsed, err := parseTarget(input)
		return connection{target: parsed}, err
	}
	host, port, err := splitHostPort(rest)
	if err != nil || !validHost(host) {
		return connection{}, errBadTarget
	}
	result := connection{target: target{host: host, port: defaultTelnetPort}, protocol: protocolTelnet}
	if port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return connection{}, errBadTarget
		}
		result.port = uint16(value)
	}
	return result, nil
}
