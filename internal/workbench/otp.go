package workbench

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"

	"motty/internal/otp"
	"motty/internal/sshconn"
	"motty/internal/vault"
)

// One-time code secrets are kept in the saved-passwords vault, one per account,
// as an otpauth:// address. The host dialog is where they are given and taken
// away; the connection (vault_ui.go) answers the server's question with them.

func (state *State) otpName(host Host) string {
	return otpSecretName(sshconn.Endpoint{Host: host.Address, Port: host.Port, User: state.login(host)})
}

// otpMayExist says whether checking this host could find a saved secret without
// opening the vault. A locked vault cannot distinguish live records from
// tombstones, so this deliberately permits a harmless false positive.
func (state *State) otpMayExist(host Host) bool {
	vaultFile := state.secrets.vault
	return vaultFile != nil && vaultFile.Len() > 0 && host.isSSH()
}

// prepareOTPState fills the editor from an already-open vault only. Opening an
// ordinary host editor must not make the system secret store display a prompt.
func (state *State) prepareOTPState(host Host) {
	editor, vaultFile := &state.editor, state.secrets.vault
	editor.otpChecked, editor.otpSaved, editor.otpRemove = true, false, false
	if !state.otpMayExist(host) {
		return
	}
	if !vaultFile.Unlocked() {
		editor.otpChecked = false
		return
	}
	_, editor.otpSaved, _ = vaultFile.Get(state.otpName(host))
}

// checkEditedHostOTP is the explicit action that may ask the system secret
// store for the vault key. It checks the original account, whose secret will be
// moved if the edited endpoint changes when the host is saved.
func (state *State) checkEditedHostOTP() bool {
	if state.hostDialogMode != hostDialogEdit {
		state.editor.otpChecked = true
		return true
	}
	host, ok := state.hostByID(state.hostDialogID)
	if !ok {
		return false
	}
	return state.checkOTP(host)
}

func (state *State) checkOTP(host Host) bool {
	editor, vaultFile := &state.editor, state.secrets.vault
	if !state.otpMayExist(host) {
		editor.otpChecked, editor.otpSaved, editor.otpRemove = true, false, false
		return true
	}
	_, found, err := vaultFile.Get(state.otpName(host))
	switch {
	case errors.Is(err, vault.ErrLocked):
		state.dialogError = state.messages.Tools.ErrOTPLocked
		return false
	case err != nil:
		state.dialogError = fmt.Sprintf(state.messages.Tools.ErrOTPCheckFmt, state.vaultErrorText(err))
		return false
	}
	editor.otpChecked, editor.otpSaved, editor.otpRemove = true, found, false
	state.dialogError = ""
	return true
}

// otpNeedsVault reports whether saving the form touches the vault.
func (state *State) otpNeedsVault(previous *Host) bool {
	editor := &state.editor
	if editor.protocol != protocolSSH {
		return false
	}
	if editor.value(fieldOTP) != "" || editor.otpRemove {
		return true
	}
	if previous != nil && editor.otpSaved {
		updated := editor.host(previous.ID)
		return state.otpName(updated) != state.otpName(*previous)
	}
	if previous != nil && !editor.otpChecked && state.otpMayExist(*previous) {
		updated := editor.host(previous.ID)
		return state.otpName(updated) != state.otpName(*previous)
	}
	return false
}

// otpPrecheck says why the secret cannot be kept, before anything is saved, so a
// host is not stored with its secret silently dropped.
func (state *State) otpPrecheck(previous *Host) string {
	if !state.otpNeedsVault(previous) {
		return ""
	}
	if previous != nil && state.editor.value(fieldOTP) == "" && !state.editor.otpRemove &&
		!state.editor.otpChecked && !state.checkOTP(*previous) {
		return state.dialogError
	}
	vaultFile := state.secrets.vault
	switch {
	case vaultFile == nil:
		return state.messages.Tools.ErrOTPNoVault
	case vaultFile.Unlocked():
		return ""
	case vaultFile.HasMasterPassword():
		return state.messages.Tools.ErrOTPLocked
	case vaultFile.Prepare() != nil:
		return state.messages.Tools.ErrOTPNoVault
	}
	return ""
}

// commitOTP applies the form's secret to the vault once the host is saved:
// store a new one, remove the old one, or move it when the account changed.
func (state *State) commitOTP(host Host, previous *Host) {
	editor := &state.editor
	vaultFile := state.secrets.vault
	if vaultFile == nil || host.Protocol != protocolSSH {
		return
	}
	name, oldName := state.otpName(host), ""
	if previous != nil {
		oldName = state.otpName(*previous)
	}
	label := state.login(host) + "@" + host.Address
	switch typed := editor.value(fieldOTP); {
	case typed != "":
		secret, err := otp.Parse(typed)
		if err != nil {
			return
		}
		if err := vaultFile.Set(name, secret.String()); err != nil {
			state.status = fmt.Sprintf(state.messages.VaultSaveFailedFmt, state.vaultErrorText(err))
			return
		}
		if oldName != "" && oldName != name {
			_ = vaultFile.Delete(oldName)
		}
		state.status = fmt.Sprintf(state.messages.Tools.OTPSavedFmt, label)
	case editor.otpRemove:
		for _, candidate := range []string{name, oldName} {
			if candidate != "" {
				_ = vaultFile.Delete(candidate)
			}
		}
		state.status = fmt.Sprintf(state.messages.Tools.OTPRemovedFmt, label)
	case oldName != "" && oldName != name && editor.otpSaved:
		value, found, err := vaultFile.Get(oldName)
		if err == nil && found && vaultFile.Set(name, value) == nil {
			_ = vaultFile.Delete(oldName)
		} else if err != nil && !errors.Is(err, vault.ErrLocked) {
			state.status = fmt.Sprintf(state.messages.VaultSaveFailedFmt, state.vaultErrorText(err))
		}
	}
}

// scanOTPImage asks for an image of a QR code and puts the secret in it into the
// form's field, where it is shown, checked and saved like a pasted address. The
// image is read once; neither it nor the text in it is kept or logged.
func (state *State) scanOTPImage() {
	m := state.messages.Tools
	paths, err := state.options.OpenFiles(app.OpenFileOptions{
		Title:   m.OTPScanTitle,
		Filters: []app.FileFilter{{Name: m.OTPScanFilter, Pattern: "*.png;*.jpg;*.jpeg;*.gif"}},
	})
	if errors.Is(err, app.ErrCanceled) || err == nil && len(paths) == 0 {
		return
	}
	if err != nil {
		state.dialogError = fmt.Sprintf(m.ErrOTPScanReadFmt, err)
		return
	}
	secrets, err := otp.FromImageFile(paths[0])
	state.takeScannedSecrets(secrets, err)
}

// pasteOTPImage reads the QR code from an image on the clipboard, as scanOTPImage
// does from a file. The clipboard is read when the button is pressed and not
// before: asking another program whether it holds an image can take a while, and a
// picture copied after the dialog opened must still be found.
func (state *State) pasteOTPImage() {
	m := state.messages.Tools
	data, err := state.options.ClipboardImagePNG()
	switch {
	case errors.Is(err, app.ErrNoImage):
		state.dialogError = m.ErrOTPPasteNoImage
		return
	case errors.Is(err, app.ErrTooLarge):
		state.dialogError = m.ErrOTPScanTooLarge
		return
	case err != nil:
		state.dialogError = fmt.Sprintf(m.ErrOTPScanReadFmt, err)
		return
	}
	secrets, err := otp.FromImage(data)
	state.takeScannedSecrets(secrets, err)
}

// takeScannedSecrets puts what a QR code carried into the form's field, or says
// why it could not.
func (state *State) takeScannedSecrets(secrets []otp.Secret, err error) {
	m := state.messages.Tools
	if err != nil {
		state.dialogError = state.otpScanErrorText(err)
		return
	}
	secret, ok := state.pickScannedSecret(secrets)
	if !ok {
		state.dialogError = fmt.Sprintf(m.ErrOTPScanManyFmt, len(secrets))
		return
	}
	state.editor.otp = text.Editor{}
	state.editor.otp.Insert(secret.String())
	state.editor.otpRemove = false
	state.dialogError = ""
	state.status = fmt.Sprintf(m.OTPScannedFmt, scannedLabel(secret))
}

// pickScannedSecret chooses the secret for this host. An export QR code holds
// every account of the authenticator, so with several the one named like the
// host's user is taken, and when none is, the choice is left to the person.
func (state *State) pickScannedSecret(secrets []otp.Secret) (otp.Secret, bool) {
	if len(secrets) == 1 {
		return secrets[0], true
	}
	user := strings.ToLower(state.editor.value(fieldUser))
	address := strings.ToLower(state.editor.value(fieldAddress))
	var found []otp.Secret
	for _, secret := range secrets {
		account := strings.ToLower(secret.Account)
		local, _, _ := strings.Cut(account, "@")
		if user != "" && (account == user || local == user) || address != "" && strings.Contains(account, address) {
			found = append(found, secret)
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	return otp.Secret{}, false
}

func scannedLabel(secret otp.Secret) string {
	switch {
	case secret.Issuer != "" && secret.Account != "":
		return secret.Issuer + " · " + secret.Account
	case secret.Issuer != "":
		return secret.Issuer
	}
	return secret.Account
}

func (state *State) otpScanErrorText(err error) string {
	m := state.messages.Tools
	switch {
	case errors.Is(err, otp.ErrImageTooLarge):
		return m.ErrOTPScanTooLarge
	case errors.Is(err, otp.ErrImageDimension):
		return m.ErrOTPScanDimension
	case errors.Is(err, otp.ErrNotImage):
		return m.ErrOTPScanNotImage
	case errors.Is(err, otp.ErrNoQRCode):
		return m.ErrOTPScanNoCode
	case errors.Is(err, otp.ErrNotOTPQRCode):
		return m.ErrOTPScanNotOTP
	case errors.Is(err, otp.ErrMigration):
		return m.ErrOTPScanMigration
	case errors.Is(err, os.ErrNotExist), errors.Is(err, os.ErrPermission):
		return fmt.Sprintf(m.ErrOTPScanReadFmt, err)
	case errors.Is(err, otp.ErrSecret), errors.Is(err, otp.ErrScheme), errors.Is(err, otp.ErrParameter), errors.Is(err, otp.ErrNoSecretKey), errors.Is(err, otp.ErrEmpty):
		return m.ErrOTPInvalid
	}
	return fmt.Sprintf(m.ErrOTPScanReadFmt, err)
}
