package workbench

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func TestRevealButtonShowsASecretAndHidesItAgainWhenTheDialogIsReopened(t *testing.T) {
	state := newTestState(t, testOptions(t))
	state.Click(hitAddHost)
	typeInto(state, hitDialogPassword, "hunter2")

	secure := func() bool {
		canvas, _ := paintedTexts(state)
		for _, hit := range canvas.Hits() {
			if hit.ID == hitDialogPassword && hit.Role == ui.RoleTextField {
				return hit.Secure
			}
		}
		t.Fatal("the password field was not painted")
		return false
	}
	if !secure() {
		t.Fatal("a password field starts out showing its text")
	}
	state.Click(revealButton(hitDialogPassword))
	if secure() {
		t.Fatal("the show button did not reveal the password")
	}
	state.Click(revealButton(hitDialogPassword))
	if !secure() {
		t.Fatal("the hide button did not mask the password")
	}

	state.Click(revealButton(hitDialogPassword))
	secure()
	state.Click(hitDialogCancel)
	paintedTexts(state)
	state.Click(hitAddHost)
	if !secure() {
		t.Fatal("a password stayed revealed after its dialog was closed")
	}
}

func TestHostFormSavesTheKeyPassphraseUnderTheKeyAndRefusesAWrongOne(t *testing.T) {
	locked := writeWorkbenchIdentity(t, "id_locked", "secret")
	options := freshOptions(t)
	options.Keystore = &memKeys{}
	options.OpenFiles = func(app.OpenFileOptions) ([]string, error) { return []string{locked}, nil }
	state := newTestState(t, options)
	state.Click(hitKeychain)
	state.Click(hitKeysImport)
	state.Click(hitHosts)

	state.Click(hitAddHost)
	typeInto(state, hitDialogLabel, "Vault")
	typeInto(state, hitDialogAddress, "vault.example.com")
	typeInto(state, hitDialogUser, "ops")
	state.Click(hitDialogAuth)
	state.Click(hitDialogAuthItemList.At(int(authPrivateKey)))
	var canvas ui.Canvas
	state.Paint(&canvas, ui.Size{Width: 1400, Height: 900})
	if _, ok := hitByID(&canvas, hitDialogPassphrase); ok {
		t.Fatal("the passphrase was asked before a key was chosen")
	}
	state.Click(hitDialogIdentity)
	state.Click(hitDialogIdentityItemList.At(int(state.identities[0].ID)))
	state.Paint(&canvas, ui.Size{Width: 1400, Height: 900})
	if _, ok := hitByID(&canvas, hitDialogPassphrase); !ok {
		t.Fatal("an encrypted key did not ask for its passphrase")
	}

	typeInto(state, hitDialogPassphrase, "not it")
	state.Click(hitDialogSubmit)
	if state.modal != modalNewHost || state.dialogError != state.messages.KeyPassphraseWrong {
		t.Fatalf("a wrong passphrase was accepted: modal %d, error %q", state.modal, state.dialogError)
	}
	if _, found, _ := state.secrets.vault.Get(passphraseSecretName(locked)); found {
		t.Fatal("a wrong passphrase reached the vault")
	}

	state.editor.passphrase = text.Editor{}
	typeInto(state, hitDialogPassphrase, "secret")
	state.Click(hitDialogSubmit)
	if state.modal != modalNone {
		t.Fatalf("the host was not saved: %q", state.dialogError)
	}
	if value, found, err := state.secrets.vault.Get(passphraseSecretName(locked)); err != nil || !found || value != "secret" {
		t.Fatalf("saved passphrase = %q, %v, %v", value, found, err)
	}
}

func TestEditHostShowsItsSavedPasswordWhenTheVaultIsOpenAndOtherwiseOnRequest(t *testing.T) {
	state := newTestState(t, testOptions(t))
	if err := state.secrets.vault.Set(prodPasswordName, "source secret"); err != nil {
		t.Fatal(err)
	}
	clickHostAction(state, hitEditHost)
	if got := state.editor.password.String(); got != "source secret" {
		t.Fatalf("an open vault left the saved password out of the form: %q", got)
	}
	state.Click(hitDialogCancel)

	state.secrets.vault.Lock()
	clickHostAction(state, hitEditHost)
	if got := state.editor.password.String(); got != "" {
		t.Fatalf("opening the dialog read a vault that was not open: %q", got)
	}
	state.Click(revealButton(hitDialogPassword))
	if got := state.editor.password.String(); got != "source secret" {
		t.Fatalf("the show button did not read the saved password: %q", got)
	}
}

func TestShowingASavedPasswordBehindAMasterPasswordUnlocksFirstAndKeepsTheDialog(t *testing.T) {
	state := newTestState(t, testOptions(t))
	if err := state.secrets.vault.Set(prodPasswordName, "source secret"); err != nil {
		t.Fatal(err)
	}
	if err := state.secrets.vault.SetMasterPassword("master password"); err != nil {
		t.Fatal(err)
	}
	state.secrets.vault.Lock()
	clickHostAction(state, hitEditHost)
	typeInto(state, hitDialogLabel, " edited")
	label := state.editor.label.String()

	state.Click(revealButton(hitDialogPassword))
	if state.modal != modalVaultUnlock {
		t.Fatalf("a locked vault was not asked to unlock: modal %d", state.modal)
	}
	state.Click(hitVaultUnlockCancel)
	if state.modal != modalNewHost || state.editor.label.String() != label {
		t.Fatalf("dismissing the unlock lost the dialog: modal %d, label %q", state.modal, state.editor.label.String())
	}

	state.Click(revealButton(hitDialogPassword))
	typeInto(state, hitVaultUnlockField, "master password")
	state.Click(hitVaultUnlockSubmit)
	if state.modal != modalNewHost || state.editor.password.String() != "source secret" {
		t.Fatalf("unlocking did not bring back the dialog with the password: modal %d, %q", state.modal, state.editor.password.String())
	}
}
