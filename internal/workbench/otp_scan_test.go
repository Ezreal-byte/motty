package workbench

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/vivaui/viva/app"
	gtext "github.com/vivaui/viva/text"
)

func writeQRImage(t *testing.T, content string) string {
	t.Helper()
	matrix, err := qrcode.NewQRCodeWriter().Encode(content, gozxing.BarcodeFormat_QR_CODE, 400, 400, nil)
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
	path := filepath.Join(t.TempDir(), "qr.png")
	if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanningAQRImageFillsTheSecretFieldAndSavingKeepsIt(t *testing.T) {
	state, _ := otpState(t)
	editProduction(t, state)
	path := writeQRImage(t, "otpauth://totp/Prod:deploy?secret="+testOTPSeed+"&issuer=Prod")
	state.options.OpenFiles = func(app.OpenFileOptions) ([]string, error) { return []string{path}, nil }
	state.Click(hitDialogOTPScan)
	if state.dialogError != "" || !strings.Contains(state.editor.value(fieldOTP), testOTPSeed) {
		t.Fatalf("error %q, field %q", state.dialogError, state.editor.value(fieldOTP))
	}
	if !strings.Contains(state.status, "Prod") {
		t.Fatalf("status %q", state.status)
	}
	state.Click(hitDialogSubmit)
	if _, found, _ := state.secrets.vault.Get(prodOTPName); !found || state.modal != modalNone {
		t.Fatalf("not saved: modal %d, error %q", state.modal, state.dialogError)
	}
}

func TestScanningSaysWhatIsWrongWithTheImage(t *testing.T) {
	state, _ := otpState(t)
	editProduction(t, state)
	m := state.messages.Tools
	scan := func(path string) string {
		state.options.OpenFiles = func(app.OpenFileOptions) ([]string, error) { return []string{path}, nil }
		state.Click(hitDialogOTPScan)
		return state.dialogError
	}
	text := filepath.Join(t.TempDir(), "notes.png")
	if err := os.WriteFile(text, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := scan(text); got != m.ErrOTPScanNotImage {
		t.Errorf("not an image: %q", got)
	}
	if got := scan(writeQRImage(t, "https://example.com")); got != m.ErrOTPScanNotOTP {
		t.Errorf("not otpauth: %q", got)
	}
	if got := scan(filepath.Join(t.TempDir(), "missing.png")); !strings.HasPrefix(got, "Could not read") {
		t.Errorf("missing file: %q", got)
	}
	if state.editor.value(fieldOTP) != "" {
		t.Fatal("a failed scan changed the field")
	}

	// An export of several accounts is used for the host whose user it names.
	entry := func(name string) []byte {
		body := append([]byte{0x0a, 20}, "12345678901234567890"...)
		body = append(body, 0x12, byte(len(name)))
		body = append(body, name...)
		return append([]byte{0x0a, byte(len(body))}, body...)
	}
	payload := append(entry("alice"), entry("deploy")...)
	path := writeQRImage(t, "otpauth-migration://offline?data="+base64.StdEncoding.EncodeToString(payload))
	if got := scan(path); got != "" || !strings.Contains(state.editor.value(fieldOTP), "deploy") {
		t.Fatalf("export of two accounts: error %q, field %q", got, state.editor.value(fieldOTP))
	}
	state.editor.user = gtext.Editor{}
	state.editor.user.Insert("zed")
	if got := scan(path); !strings.Contains(got, "2 accounts") {
		t.Fatalf("export naming nobody: %q", got)
	}
}

func TestPastingAQRImageFromTheClipboardFillsTheField(t *testing.T) {
	state, _ := otpState(t)
	editProduction(t, state)
	m := state.messages.Tools
	data, err := os.ReadFile(writeQRImage(t, "otpauth://totp/Prod:deploy?secret="+testOTPSeed+"&issuer=Prod"))
	if err != nil {
		t.Fatal(err)
	}

	// Whatever the clipboard holds is read when the button is pressed, so a picture
	// copied after the dialog opened is found.
	state.options.ClipboardImagePNG = func() ([]byte, error) { return nil, app.ErrNoImage }
	state.Click(hitDialogOTPPaste)
	if state.dialogError != m.ErrOTPPasteNoImage || state.editor.value(fieldOTP) != "" {
		t.Fatalf("an empty clipboard: error %q, field %q", state.dialogError, state.editor.value(fieldOTP))
	}
	state.options.ClipboardImagePNG = func() ([]byte, error) { return data, nil }
	state.Click(hitDialogOTPPaste)
	if state.dialogError != "" || !strings.Contains(state.editor.value(fieldOTP), testOTPSeed) {
		t.Fatalf("error %q, field %q", state.dialogError, state.editor.value(fieldOTP))
	}
	if !strings.Contains(state.status, "Prod") {
		t.Fatalf("status %q", state.status)
	}
	state.Click(hitDialogSubmit)
	if _, found, _ := state.secrets.vault.Get(prodOTPName); !found || state.modal != modalNone {
		t.Fatalf("not saved: modal %d, error %q", state.modal, state.dialogError)
	}
}

func TestPastingAnImageThatIsNotAQRCodeSaysSoAndChangesNothing(t *testing.T) {
	state, _ := otpState(t)
	editProduction(t, state)
	m := state.messages.Tools
	blank := image.NewGray(image.Rect(0, 0, 200, 200))
	for i := range blank.Pix {
		blank.Pix[i] = 255
	}
	var out bytes.Buffer
	if err := png.Encode(&out, blank); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		read func() ([]byte, error)
		want string
	}{
		"a picture without a code":    {func() ([]byte, error) { return out.Bytes(), nil }, m.ErrOTPScanNoCode},
		"bytes that are not an image": {func() ([]byte, error) { return []byte("plain text"), nil }, m.ErrOTPScanNotImage},
		"an image above the cap":      {func() ([]byte, error) { return nil, app.ErrTooLarge }, m.ErrOTPScanTooLarge},
	}
	for name, test := range cases {
		state.options.ClipboardImagePNG = test.read
		state.Click(hitDialogOTPPaste)
		if state.dialogError != test.want {
			t.Errorf("%s: error %q, want %q", name, state.dialogError, test.want)
		}
	}
	if state.editor.value(fieldOTP) != "" {
		t.Fatal("a failed paste changed the field")
	}
}
