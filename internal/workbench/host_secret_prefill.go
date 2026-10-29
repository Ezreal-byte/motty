package workbench

import (
	"errors"

	"github.com/vivaui/viva/ui"

	"motty/internal/vault"
)

// A host dialog shows the secrets already saved for the host, so editing a host
// does not look as if its password had been lost. Opening the dialog reads them
// only from a vault that is already open: it must not be what makes the system
// ask for the vault's key. Otherwise they stay out of the form until the user
// asks to see them with the field's button, which unlocks the vault if it must.

// savedHostSecretName is the vault name of the secret a form field stands for,
// and false when there is none to look up. The password belongs to the host as
// it was saved, not as it is being edited: an edited address is not yet where
// the secret is kept.
func (state *State) savedHostSecretName(field hostField) (string, bool) {
	switch field {
	case fieldPassword:
		host, ok := state.hostByID(state.hostDialogID)
		if !ok || state.hostDialogMode != hostDialogEdit {
			return "", false
		}
		return state.hostPasswordSecretName(host)
	case fieldPassphrase:
		if !state.hostAsksKeyPassphrase() {
			return "", false
		}
		identity, ok := state.identityForFingerprint(state.editor.identity)
		if !ok {
			return "", false
		}
		return passphraseSecretName(identity.Path), true
	}
	return "", false
}

// loadSavedHostSecret puts the saved secret of a form field into it. With
// openOnly it reads nothing from a vault that is not open yet. An ErrLocked
// result means a master password must be typed first; any other failure leaves
// the field empty, which saving reads as "keep what is saved".
func (state *State) loadSavedHostSecret(field hostField, openOnly bool) error {
	name, ok := state.savedHostSecretName(field)
	vaultFile := state.secrets.vault
	// An empty vault has nothing to read; asking it would still open the system
	// store for a key that does not exist yet.
	if !ok || vaultFile == nil || vaultFile.Len() == 0 || openOnly && !vaultFile.Unlocked() {
		return nil
	}
	value, found, err := vaultFile.Get(name)
	if err != nil || !found {
		return err
	}
	editor := state.editor.field(field)
	if editor.String() == "" {
		editor.Insert(value)
	}
	return nil
}

// prefillHostSecrets fills the password of the host being edited and the
// passphrase of its key, if the vault opens without being asked.
func (state *State) prefillHostSecrets() {
	_ = state.loadSavedHostSecret(fieldPassword, true)
	state.prefillKeyPassphrase()
}

// prefillKeyPassphrase does the same for the key the form has just pinned. It
// asks once per key, so clearing the field is not undone by the next click.
func (state *State) prefillKeyPassphrase() {
	editor := &state.editor
	if !state.hostAsksKeyPassphrase() || editor.passphraseFor == editor.identity {
		return
	}
	editor.passphraseFor = editor.identity
	if editor.passphrase.String() == "" {
		_ = state.loadSavedHostSecret(fieldPassphrase, true)
	}
}

// revealSavedHostSecret answers the show button of an empty field that has a
// saved secret behind it. A vault behind a master password is unlocked first,
// and the dialog comes back with the secret in it, or just as it was when the
// unlock is dismissed.
func (state *State) revealSavedHostSecret(fieldID ui.ID) {
	if state.modal != modalNewHost || (fieldID != hitDialogPassword && fieldID != hitDialogPassphrase) {
		return
	}
	field := fieldForDialogHit(fieldID)
	if state.editor.field(field).String() != "" {
		return
	}
	if err := state.loadSavedHostSecret(field, false); !errors.Is(err, vault.ErrLocked) {
		return
	}
	restore := func() {
		state.modal = modalNewHost
		state.dialogError = ""
	}
	state.openVaultUnlock(nil, func() {
		restore()
		_ = state.loadSavedHostSecret(field, false)
		state.reveal.show(revealButton(fieldID))
	})
	state.secrets.back = restore
}
