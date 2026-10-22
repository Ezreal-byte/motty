package otp

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// qrPNG renders text as a QR code PNG, the way an authenticator app shows it.
func qrPNG(t *testing.T, text string, size int) []byte {
	t.Helper()
	matrix, err := qrcode.NewQRCodeWriter().Encode(text, gozxing.BarcodeFormat_QR_CODE, size, size, nil)
	if err != nil {
		t.Fatal(err)
	}
	picture := image.NewGray(image.Rect(0, 0, matrix.GetWidth(), matrix.GetHeight()))
	for y := 0; y < matrix.GetHeight(); y++ {
		for x := 0; x < matrix.GetWidth(); x++ {
			if !matrix.Get(x, y) {
				picture.Pix[y*picture.Stride+x] = 255
			}
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, picture); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestFromImageOtpauth(t *testing.T) {
	const address = "otpauth://totp/Example:alice@example.com?secret=GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ&issuer=Example&digits=8"
	secrets, err := FromImage(qrPNG(t, address, 300))
	if err != nil || len(secrets) != 1 {
		t.Fatalf("FromImage = %v, %v", secrets, err)
	}
	got := secrets[0]
	if got.Issuer != "Example" || got.Account != "alice@example.com" || got.Digits != 8 || string(got.Key) != "12345678901234567890" {
		t.Fatalf("secret = %+v", got)
	}
}

func TestFromImageFailures(t *testing.T) {
	blank := image.NewGray(image.Rect(0, 0, 200, 200))
	for i := range blank.Pix {
		blank.Pix[i] = 255
	}
	var blankPNG bytes.Buffer
	if err := png.Encode(&blankPNG, blank); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"not an image", []byte("hello, this is text"), ErrNotImage},
		{"truncated png", qrPNG(t, "otpauth://totp/a?secret=GEZDGNBVGY3TQOJQ", 200)[:60], ErrNotImage},
		{"no code", blankPNG.Bytes(), ErrNoQRCode},
		{"not otpauth", qrPNG(t, "https://example.com/", 200), ErrNotOTPQRCode},
		{"bad secret", qrPNG(t, "otpauth://totp/a?secret=!!!", 200), ErrSecret},
		{"oversize", make([]byte, maxImageBytes+1), ErrImageTooLarge},
	}
	for _, test := range cases {
		if _, err := FromImage(test.data); !errors.Is(err, test.want) {
			t.Errorf("%s: err = %v, want %v", test.name, err, test.want)
		}
	}
}

func TestFromImageFileErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := FromImageFile(filepath.Join(dir, "missing.png")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file: %v", err)
	}
	big := filepath.Join(dir, "big.png")
	if err := os.WriteFile(big, make([]byte, maxImageBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := FromImageFile(big); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("big file: %v", err)
	}
}

// pb builds protobuf fields for the migration tests.
type pb []byte

func (b pb) varint(number, value uint64) pb { return b.raw(number<<3, value) }
func (b pb) raw(tag, value uint64) pb {
	b = appendVarint(b, tag)
	return appendVarint(b, value)
}
func (b pb) bytes(number uint64, value []byte) pb {
	b = appendVarint(b, number<<3|2)
	b = appendVarint(b, uint64(len(value)))
	return append(b, value...)
}
func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func TestMigration(t *testing.T) {
	first := pb{}.bytes(1, []byte("12345678901234567890")).bytes(2, []byte("Example:alice")).bytes(3, []byte("Example")).
		varint(4, 2).varint(5, 2).varint(6, 2)
	second := pb{}.bytes(1, []byte("abcdefghijklmnopqrst")).bytes(2, []byte("bob")).varint(6, 1).varint(7, 5)
	payload := pb{}.bytes(1, first).bytes(1, second).varint(2, 1).varint(3, 1)
	address := "otpauth-migration://offline?data=" + base64.StdEncoding.EncodeToString(payload)

	secrets, err := FromImage(qrPNG(t, address, 400))
	if err != nil || len(secrets) != 2 {
		t.Fatalf("FromImage = %v, %v", secrets, err)
	}
	if a := secrets[0]; a.Kind != TOTP || a.Issuer != "Example" || a.Account != "alice" || a.Algorithm != "SHA256" || a.Digits != 8 {
		t.Fatalf("first = %+v", a)
	}
	if b := secrets[1]; b.Kind != HOTP || b.Account != "bob" || b.Counter != 5 || string(b.Key) != "abcdefghijklmnopqrst" {
		t.Fatalf("second = %+v", b)
	}
}

func TestMigrationMalformed(t *testing.T) {
	good := pb{}.bytes(1, []byte("12345678901234567890"))
	cases := map[string]string{
		"no data":         "otpauth-migration://offline",
		"not base64":      "otpauth-migration://offline?data=@@@",
		"truncated field": "otpauth-migration://offline?data=" + base64.StdEncoding.EncodeToString(pb{}.bytes(1, good)[:10]),
		"short key":       "otpauth-migration://offline?data=" + base64.StdEncoding.EncodeToString(pb{}.bytes(1, pb{}.bytes(1, []byte("short")))),
		"md5":             "otpauth-migration://offline?data=" + base64.StdEncoding.EncodeToString(pb{}.bytes(1, append(append(pb{}, good...), pb{}.varint(4, 4)...))),
	}
	for name, address := range cases {
		if _, err := ParseMigration(address); !errors.Is(err, ErrMigration) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
