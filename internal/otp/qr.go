package otp

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"image"
	_ "image/gif"  // register the decoders FromImageFile accepts
	_ "image/jpeg" //
	_ "image/png"  //
	"net/url"
	"os"
	"strings"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// Errors FromImageFile returns besides those of Parse.
var (
	ErrImageTooLarge  = errors.New("otp: the image file is too large")
	ErrImageDimension = errors.New("otp: the image has too many pixels")
	ErrNotImage       = errors.New("otp: not a PNG, JPEG or GIF image")
	ErrNoQRCode       = errors.New("otp: no QR code found in the image")
	ErrNotOTPQRCode   = errors.New("otp: the QR code is not an otpauth:// address")
	ErrMigration      = errors.New("otp: the authenticator export QR code is damaged or uses an unsupported algorithm")
)

const (
	maxImageBytes       = 10 << 20
	maxImagePixels      = 16 << 20
	maxMigrationEntries = 256
)

const migrationPrefix = "otpauth-migration:"

// FromImageFile reads the QR code in a PNG, JPEG or GIF file and returns the
// secrets it carries: one for an otpauth:// address, one or more for the export
// QR code of Google Authenticator. The file is read once and neither it nor the
// text in it is kept.
func FromImageFile(path string) ([]Secret, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxImageBytes {
		return nil, ErrImageTooLarge
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return FromImage(data)
}

// FromImage is FromImageFile for the bytes of an image.
func FromImage(data []byte) ([]Secret, error) {
	if len(data) > maxImageBytes {
		return nil, ErrImageTooLarge
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrNotImage
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return nil, ErrImageDimension
	}
	picture, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrNotImage
	}
	bitmap, err := gozxing.NewBinaryBitmapFromImage(picture)
	if err != nil {
		return nil, ErrNoQRCode
	}
	hints := map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}
	result, err := qrcode.NewQRCodeReader().Decode(bitmap, hints)
	if err != nil {
		return nil, ErrNoQRCode
	}
	return FromQRText(result.GetText())
}

// FromQRText turns the text of a QR code into secrets.
func FromQRText(text string) ([]Secret, error) {
	text = strings.TrimSpace(text)
	lower := strings.ToLower(text)
	switch {
	case strings.HasPrefix(lower, migrationPrefix):
		return ParseMigration(text)
	case strings.HasPrefix(lower, "otpauth:"):
		secret, err := Parse(text)
		if err != nil {
			return nil, err
		}
		return []Secret{secret}, nil
	}
	return nil, ErrNotOTPQRCode
}

// ParseMigration reads an otpauth-migration://offline?data=... address, the
// export of Google Authenticator: a base64 protobuf MigrationPayload holding
// repeated OtpParameters (secret 1, name 2, issuer 3, algorithm 4, digits 5,
// type 6, counter 7).
func ParseMigration(address string) ([]Secret, error) {
	parsed, err := url.Parse(strings.TrimSpace(address))
	if err != nil {
		return nil, ErrMigration
	}
	encoded := strings.ReplaceAll(parsed.Query().Get("data"), " ", "+") // a '+' in a query reads as a blank
	encoded = strings.TrimRight(encoded, "=")
	payload, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		if payload, err = base64.RawURLEncoding.DecodeString(encoded); err != nil {
			return nil, ErrMigration
		}
	}
	var secrets []Secret
	err = protoFields(payload, func(number uint64, wire int, value []byte, _ uint64) error {
		if number != 1 || wire != 2 {
			return nil
		}
		if len(secrets) == maxMigrationEntries {
			return ErrMigration
		}
		secret, err := migrationEntry(value)
		if err != nil {
			return err
		}
		secrets = append(secrets, secret)
		return nil
	})
	if err != nil {
		return nil, ErrMigration
	}
	if len(secrets) == 0 {
		return nil, ErrMigration
	}
	return secrets, nil
}

func migrationEntry(data []byte) (Secret, error) {
	secret := Secret{Kind: TOTP, Digits: defaultDigits, Period: defaultPeriod, Algorithm: "SHA1"}
	var name string
	err := protoFields(data, func(number uint64, wire int, value []byte, number64 uint64) error {
		switch {
		case number == 1 && wire == 2:
			secret.Key = append([]byte(nil), value...)
		case number == 2 && wire == 2:
			name = string(value)
		case number == 3 && wire == 2:
			secret.Issuer = string(value)
		case number == 4 && wire == 0:
			switch number64 {
			case 0, 1:
			case 2:
				secret.Algorithm = "SHA256"
			case 3:
				secret.Algorithm = "SHA512"
			default:
				return ErrMigration // MD5
			}
		case number == 5 && wire == 0:
			switch number64 {
			case 0, 1:
			case 2:
				secret.Digits = 8
			case 3:
				secret.Digits = 7
			default:
				return ErrMigration
			}
		case number == 6 && wire == 0:
			switch number64 {
			case 1:
				secret.Kind = HOTP
			case 0, 2:
			default:
				return ErrMigration
			}
		case number == 7 && wire == 0:
			secret.Counter = number64
		}
		return nil
	})
	if err != nil {
		return Secret{}, err
	}
	if len(secret.Key) < minKeyBytes || len(secret.Key) > maxKeyBytes {
		return Secret{}, ErrMigration
	}
	if issuer, account, found := strings.Cut(name, ":"); found {
		if secret.Issuer == "" {
			secret.Issuer = strings.TrimSpace(issuer)
		}
		name = account
	}
	secret.Account = strings.TrimSpace(name)
	return secret, nil
}

// protoFields calls visit for every field of one protobuf message. A
// length-delimited field passes its bytes, a varint field its value.
func protoFields(data []byte, visit func(number uint64, wire int, value []byte, varint uint64) error) error {
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 {
			return ErrMigration
		}
		data = data[n:]
		number, wire := tag>>3, int(tag&7)
		var value []byte
		var varint uint64
		switch wire {
		case 0:
			if varint, n = binary.Uvarint(data); n <= 0 {
				return ErrMigration
			}
			data = data[n:]
		case 1:
			if len(data) < 8 {
				return ErrMigration
			}
			data = data[8:]
		case 2:
			size, n := binary.Uvarint(data)
			if n <= 0 || size > uint64(len(data)-n) {
				return ErrMigration
			}
			value, data = data[n:n+int(size)], data[n+int(size):]
		case 5:
			if len(data) < 4 {
				return ErrMigration
			}
			data = data[4:]
		default:
			return ErrMigration
		}
		if err := visit(number, wire, value, varint); err != nil {
			return err
		}
	}
	return nil
}
