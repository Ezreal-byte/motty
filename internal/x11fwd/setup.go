package x11fwd

import (
	"encoding/binary"
	"errors"
	"io"
)

// Limits on the authorization fields of a setup packet. A cookie is 16 bytes;
// these only stop a hostile client from making us buffer without bound.
const (
	maxAuthName = 256
	maxAuthData = 1024
)

var errSetup = errors.New("x11: malformed connection setup")

// Setup is the first packet an X client sends: byte order, protocol version and
// the authorization it presents.
type Setup struct {
	Order        byte // 'B' or 'l'
	Major, Minor uint16
	AuthName     string
	AuthData     []byte
}

func pad4(n int) int { return (n + 3) &^ 3 }

func (setup Setup) order() binary.ByteOrder {
	if setup.Order == 'B' {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

// ReadSetup reads exactly one setup packet from r, consuming nothing after it.
func ReadSetup(r io.Reader) (Setup, error) {
	var header [12]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Setup{}, err
	}
	setup := Setup{Order: header[0]}
	if setup.Order != 'B' && setup.Order != 'l' {
		return Setup{}, errSetup
	}
	order := setup.order()
	setup.Major, setup.Minor = order.Uint16(header[2:]), order.Uint16(header[4:])
	nameLength, dataLength := int(order.Uint16(header[6:])), int(order.Uint16(header[8:]))
	if nameLength > maxAuthName || dataLength > maxAuthData {
		return Setup{}, errSetup
	}
	body := make([]byte, pad4(nameLength)+pad4(dataLength))
	if _, err := io.ReadFull(r, body); err != nil {
		return Setup{}, err
	}
	setup.AuthName = string(body[:nameLength])
	setup.AuthData = append([]byte(nil), body[pad4(nameLength):pad4(nameLength)+dataLength]...)
	return setup, nil
}

// Marshal encodes the packet in its own byte order.
func (setup Setup) Marshal() []byte {
	out := make([]byte, 12+pad4(len(setup.AuthName))+pad4(len(setup.AuthData)))
	order := setup.order()
	out[0] = setup.Order
	order.PutUint16(out[2:], setup.Major)
	order.PutUint16(out[4:], setup.Minor)
	order.PutUint16(out[6:], uint16(len(setup.AuthName)))
	order.PutUint16(out[8:], uint16(len(setup.AuthData)))
	copy(out[12:], setup.AuthName)
	copy(out[12+pad4(len(setup.AuthName)):], setup.AuthData)
	return out
}
