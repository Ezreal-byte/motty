// Package vault keeps the secrets a user chose to save (SSH passwords and key
// passphrases) in one encrypted file.
//
// The records are sealed with AES-256-GCM under a random 32-byte data key. Where
// that key lives is the user's choice. By default it is in the system's own
// secret store (Keychain, Credential Manager, Secret Service), which opens it for
// this application without a prompt. With a master password the file instead
// holds the key wrapped by a key derived from the password with Argon2id, the
// system store holds nothing, and the vault stays locked until the password is
// typed. Record names are hidden too: a record's id is a keyed hash of its name
// and the name is inside the sealed part.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"

	"golang.org/x/crypto/argon2"
)

// The service and account the data key is kept under in the system's store.
const (
	KeyService = "motty vault"
	KeyAccount = "data key"
)

// MinMasterPasswordLength is the shortest master password the vault accepts.
const MinMasterPasswordLength = 8

// Errors callers act on; test with errors.Is.
var (
	// ErrLocked means a master password protects the vault and has not been typed.
	ErrLocked = errors.New("vault: locked")
	// ErrWrongPassword means the master password did not open the vault.
	ErrWrongPassword = errors.New("vault: wrong master password")
	// ErrWeakPassword means the new master password is too short.
	ErrWeakPassword = errors.New("vault: the master password is too short")
	// ErrKeyLost means the file holds records but the system store has lost the
	// key that opens them. Clear the vault to start again.
	ErrKeyLost = errors.New("vault: the key for the saved secrets is gone")
	// ErrUnavailable means there is no master password and the system has no
	// secret store to keep the key in, so nothing can be saved.
	ErrUnavailable = errors.New("vault: no secret store to keep the key in; set a master password")
	// ErrCorrupt means the file is not a vault, or a record was changed.
	ErrCorrupt = errors.New("vault: the file is damaged")

	// ErrKeystoreNotFound, ErrKeystoreUnavailable and ErrKeystoreDenied are what a
	// Keystore reports for a missing key, a system without a store, and a refusal.
	ErrKeystoreNotFound    = errors.New("vault: the key is not in the secret store")
	ErrKeystoreUnavailable = errors.New("vault: the system has no secret store")
	ErrKeystoreDenied      = errors.New("vault: the secret store refused access")
)

// Keystore is the system's store for a small secret: what the data key lives in
// when there is no master password.
type Keystore interface {
	Store(service, account string, secret []byte) error
	Load(service, account string) ([]byte, error)
	Delete(service, account string) error
}

// kdfParams are the Argon2id costs the wrapped key was made with. They are in the
// file, so a later release can raise the defaults and still open old vaults.
type kdfParams struct {
	Name      string `json:"name"`
	Time      uint32 `json:"time"`
	MemoryKiB uint32 `json:"memory_kib"`
	Threads   uint8  `json:"threads"`
	Salt      []byte `json:"salt"`
}

type record struct {
	ID   []byte `json:"id"`
	Data []byte `json:"data"`
}

type fileData struct {
	Version    int        `json:"version"`
	KDF        *kdfParams `json:"kdf,omitempty"`
	WrappedKey []byte     `json:"wrapped_key,omitempty"`
	Records    []record   `json:"records"`
}

const fileVersion = 1

// UseCheapKDFForTests makes every new master password cost a few milliseconds
// to derive instead of the production tens of them, for the test binaries of
// packages that create vaults. It has no effect on files that already exist,
// whose cost is stored in them, and nothing in the program calls it.
func UseCheapKDFForTests() {
	defaultKDF = kdfParams{Name: "argon2id", Time: 1, MemoryKiB: 64, Threads: 1}
}

// defaultKDF is the cost of a new master password. A variable so tests can use a
// cheap one; it is never read when an existing file is opened.
var defaultKDF = kdfParams{Name: "argon2id", Time: 3, MemoryKiB: 64 * 1024, Threads: 4}

const (
	keySize        = 32
	wrapAssociated = "motty vault data key"
	idContext      = "motty vault record id\x00"
	maxFileSize    = 8 << 20
	maxRecords     = 4096
	// maxFileRecords bounds the file itself: live secrets are limited to
	// maxRecords, and tombstones may bring the total up to this.
	maxFileRecords = 2 * maxRecords
)

// Vault is the file and what is known about opening it.
type Vault struct {
	path string
	keys Keystore

	mu      sync.Mutex
	file    fileData
	key     []byte
	stamper func() (string, error)
}

// Secret is one saved secret as it travels between devices: its name, its value
// (empty for a deletion), the version stamp that orders it against other copies
// of the same name, and whether it is a tombstone.
type Secret struct {
	Name    string `json:"name"`
	Value   string `json:"value,omitempty"`
	Stamp   string `json:"stamp"`
	Deleted bool   `json:"deleted,omitempty"`
}

// Change is one local mutation in an atomic Apply call. Delete ignores Value;
// otherwise Value replaces the secret under Name.
type Change struct {
	Name   string
	Value  string
	Delete bool
}

// SetStamper sets the source of version stamps. Stamps are opaque strings that
// compare in time order as plain strings; the vault never makes one itself. With
// a stamper, every change is stamped and Delete leaves a tombstone so the
// deletion can reach other devices. With nil the vault stamps nothing and Delete
// removes the record outright.
func (vault *Vault) SetStamper(next func() (string, error)) {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	vault.stamper = next
}

// stamp issues a fresh stamp, or "" when there is no stamper.
func (vault *Vault) stamp() (string, error) {
	if vault.stamper == nil {
		return "", nil
	}
	return vault.stamper()
}

// openAll opens every record of the file, in file order.
func (vault *Vault) openAll() ([]plainRecord, error) {
	plains := make([]plainRecord, len(vault.file.Records))
	for index, entry := range vault.file.Records {
		plain, err := open(vault.key, entry)
		if err != nil {
			return nil, err
		}
		plains[index] = plain
	}
	return plains, nil
}

// Open reads the vault at path. A file that does not exist is an empty vault; one
// that is not a vault is ErrCorrupt. Nothing is asked of the system's store yet.
func Open(path string, keys Keystore) (*Vault, error) {
	vault := &Vault{path: path, keys: keys, file: fileData{Version: fileVersion}}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return vault, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("%w: %d bytes", ErrCorrupt, info.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file fileData
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if file.Version != fileVersion {
		return nil, fmt.Errorf("%w: version %d", ErrCorrupt, file.Version)
	}
	if len(file.Records) > maxFileRecords || (file.KDF == nil) != (len(file.WrappedKey) == 0) {
		return nil, fmt.Errorf("%w: inconsistent contents", ErrCorrupt)
	}
	if file.KDF != nil && (file.KDF.Name != "argon2id" || file.KDF.Time == 0 || file.KDF.Time > 64 || file.KDF.MemoryKiB < 8 ||
		file.KDF.MemoryKiB > 1<<21 || file.KDF.Threads == 0 || len(file.KDF.Salt) < 8) {
		return nil, fmt.Errorf("%w: unusable key derivation settings", ErrCorrupt)
	}
	vault.file = file
	return vault, nil
}

// HasMasterPassword reports whether a master password protects the vault.
func (vault *Vault) HasMasterPassword() bool {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	return vault.file.KDF != nil
}

// Len is how many secrets are saved; tombstones are not secrets. While the vault
// is locked the records cannot be told apart, so Len then counts them all.
func (vault *Vault) Len() int {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if vault.key == nil {
		return len(vault.file.Records)
	}
	count := 0
	for _, entry := range vault.file.Records {
		if plain, err := open(vault.key, entry); err != nil || !plain.Deleted {
			count++ // a damaged record still occupies a place
		}
	}
	return count
}

// Unlocked reports whether the vault can be read and written right now.
func (vault *Vault) Unlocked() bool {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	return vault.key != nil
}

// Prepare makes the vault usable if that needs nobody: with no master password it
// takes the data key from the system's store. It returns ErrLocked when a master
// password is needed, ErrKeyLost when records exist but their key is gone, and
// ErrUnavailable when there is no store. An empty vault with a store is ready:
// the key is made when the first secret is saved.
func (vault *Vault) Prepare() error {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	return vault.prepare(false)
}

func (vault *Vault) prepare(create bool) error {
	if vault.key != nil {
		return nil
	}
	if vault.file.KDF != nil {
		return ErrLocked
	}
	if vault.keys == nil {
		return ErrUnavailable
	}
	key, err := vault.keys.Load(KeyService, KeyAccount)
	switch {
	case err == nil && len(key) == keySize:
		vault.key = key
		return nil
	case err == nil:
		return fmt.Errorf("%w: the stored key has the wrong size", ErrCorrupt)
	case errors.Is(err, ErrKeystoreUnavailable):
		return ErrUnavailable
	case errors.Is(err, ErrKeystoreDenied):
		return err
	case !errors.Is(err, ErrKeystoreNotFound):
		return err
	}
	if len(vault.file.Records) > 0 {
		return ErrKeyLost
	}
	if !create {
		return nil
	}
	key = make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return err
	}
	if err := vault.keys.Store(KeyService, KeyAccount, key); err != nil {
		if errors.Is(err, ErrKeystoreUnavailable) {
			return ErrUnavailable
		}
		return err
	}
	vault.key = key
	return nil
}

// Unlock opens a vault protected by a master password.
func (vault *Vault) Unlock(password string) error {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if vault.file.KDF == nil {
		return nil
	}
	key, err := unwrapKey(vault.file, password)
	if err != nil {
		return err
	}
	vault.key = key
	return nil
}

// Lock forgets the data key: a vault with a master password needs it again, and
// one without reads the system's store on next use.
func (vault *Vault) Lock() {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	clear(vault.key)
	vault.key = nil
}

// Get returns the secret saved under name, and whether there is one. It needs the
// vault to be unlocked (ErrLocked) and a record that has not been changed.
func (vault *Vault) Get(name string) (string, bool, error) {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if err := vault.prepare(false); err != nil {
		return "", false, err
	}
	if vault.key == nil {
		return "", false, nil // an empty vault with a store, before its key exists
	}
	id := recordID(vault.key, name)
	for _, entry := range vault.file.Records {
		if hmac.Equal(entry.ID, id) {
			plain, err := open(vault.key, entry)
			if err != nil {
				return "", false, err
			}
			if plain.Deleted {
				return "", false, nil
			}
			return plain.Value, true, nil
		}
	}
	return "", false, nil
}

// Set saves a secret under name, replacing one of the same name, and writes the
// file. A secret that already holds exactly value is left alone, stamp included.
// A deleted name comes back to life with a new stamp.
func (vault *Vault) Set(name, value string) error {
	return vault.Apply([]Change{{Name: name, Value: value}})
}

// Apply validates and seals every local change before writing the vault once.
// If any change fails, none of them become visible.
func (vault *Vault) Apply(changes []Change) error {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if len(changes) == 0 {
		return nil
	}
	create := false
	for _, change := range changes {
		if change.Name == "" {
			return errors.New("vault: a secret needs a name")
		}
		create = create || !change.Delete
	}
	if err := vault.prepare(create); err != nil {
		return err
	}
	if vault.key == nil {
		return nil // deleting from an empty vault
	}
	plains, err := vault.openAll()
	if err != nil {
		return err
	}
	records := append([]record(nil), vault.file.Records...)
	positions := make(map[string]int, len(plains))
	live := 0
	for index, plain := range plains {
		positions[plain.Name] = index
		if !plain.Deleted {
			live++
		}
	}
	changed := false
	for _, change := range changes {
		position, known := positions[change.Name]
		if change.Delete {
			if !known || plains[position].Deleted {
				continue
			}
			if vault.stamper == nil {
				records = slices.Delete(records, position, position+1)
				plains = slices.Delete(plains, position, position+1)
				delete(positions, change.Name)
				for name, at := range positions {
					if at > position {
						positions[name] = at - 1
					}
				}
			} else {
				stamp, err := vault.stamp()
				if err != nil {
					return err
				}
				plain := plainRecord{Name: change.Name, Deleted: true, Stamp: stamp}
				sealed, err := seal(vault.key, plain)
				if err != nil {
					return err
				}
				records[position], plains[position] = sealed, plain
			}
			live--
			changed = true
			continue
		}

		if known && !plains[position].Deleted && plains[position].Value == change.Value {
			continue
		}
		adding := !known || plains[position].Deleted
		if adding && live >= maxRecords || !known && len(records) >= maxFileRecords {
			return errors.New("vault: too many saved secrets")
		}
		stamp, err := vault.stamp()
		if err != nil {
			return err
		}
		plain := plainRecord{Name: change.Name, Value: change.Value, Stamp: stamp}
		sealed, err := seal(vault.key, plain)
		if err != nil {
			return err
		}
		if known {
			records[position], plains[position] = sealed, plain
		} else {
			positions[change.Name] = len(records)
			records, plains = append(records, sealed), append(plains, plain)
		}
		if adding {
			live++
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return vault.write(vault.file.KDF, vault.file.WrappedKey, records)
}

// Delete forgets the secret saved under name; there being none is not an error.
// With a stamper the record becomes a tombstone, so that the deletion can be
// merged into other devices; deleting a tombstone does nothing.
func (vault *Vault) Delete(name string) error {
	return vault.Apply([]Change{{Name: name, Delete: true}})
}

// Export returns every record, tombstones included, sorted by name. With a
// stamper, secrets that have no stamp yet (saved before stamps existed) are
// stamped first, so what is exported always carries one. It needs the vault
// unlocked.
func (vault *Vault) Export() ([]Secret, error) {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if err := vault.prepare(false); err != nil {
		return nil, err
	}
	if vault.key == nil {
		return nil, nil
	}
	plains, err := vault.openAll()
	if err != nil {
		return nil, err
	}
	if vault.stamper != nil {
		records := append([]record(nil), vault.file.Records...)
		order := make([]int, len(plains))
		for index := range order {
			order[index] = index
		}
		sort.Slice(order, func(a, b int) bool { return plains[order[a]].Name < plains[order[b]].Name })
		changed := false
		for _, index := range order {
			if plains[index].Deleted || plains[index].Stamp != "" {
				continue
			}
			stamp, err := vault.stamper()
			if err != nil {
				return nil, err
			}
			plains[index].Stamp = stamp
			if records[index], err = seal(vault.key, plains[index]); err != nil {
				return nil, err
			}
			changed = true
		}
		if changed {
			if err := vault.write(vault.file.KDF, vault.file.WrappedKey, records); err != nil {
				return nil, err
			}
		}
	}
	var secrets []Secret
	for _, plain := range plains {
		secrets = append(secrets, Secret{Name: plain.Name, Value: plain.Value, Stamp: plain.Stamp, Deleted: plain.Deleted})
	}
	sort.Slice(secrets, func(a, b int) bool { return secrets[a].Name < secrets[b].Name })
	return secrets, nil
}

// Merge takes in secrets received from another device. A received entry wins when
// the name is unknown here or its stamp is greater than the local one; equal or
// older entries, and entries without a name or a stamp, are ignored, as are
// entries that would exceed the vault's limits. Everything accepted is written at
// once, and returned. Merge never makes a stamp.
func (vault *Vault) Merge(received []Secret) ([]Secret, error) {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if err := vault.prepare(true); err != nil {
		return nil, err
	}
	plains, err := vault.openAll()
	if err != nil {
		return nil, err
	}
	records := append([]record(nil), vault.file.Records...)
	positions := make(map[string]int, len(records))
	live := 0
	for index, entry := range records {
		positions[string(entry.ID)] = index
		if !plains[index].Deleted {
			live++
		}
	}
	var applied []Secret
	for _, secret := range received {
		if secret.Name == "" || secret.Stamp == "" {
			continue
		}
		plain := plainRecord{Name: secret.Name, Value: secret.Value, Stamp: secret.Stamp, Deleted: secret.Deleted}
		if plain.Deleted {
			plain.Value = ""
		}
		id := recordID(vault.key, secret.Name)
		position, known := positions[string(id)]
		if known {
			if plains[position].Stamp >= secret.Stamp {
				continue
			}
			if plains[position].Deleted && !plain.Deleted && live >= maxRecords {
				continue
			}
		} else if len(records) >= maxFileRecords || (!plain.Deleted && live >= maxRecords) {
			continue
		}
		sealed, err := seal(vault.key, plain)
		if err != nil {
			return nil, err
		}
		if known {
			if !plains[position].Deleted {
				live--
			}
			records[position], plains[position] = sealed, plain
		} else {
			positions[string(id)] = len(records)
			records, plains = append(records, sealed), append(plains, plain)
		}
		if !plain.Deleted {
			live++
		}
		applied = append(applied, Secret{Name: plain.Name, Value: plain.Value, Stamp: plain.Stamp, Deleted: plain.Deleted})
	}
	if len(applied) == 0 {
		return nil, nil
	}
	if err := vault.write(vault.file.KDF, vault.file.WrappedKey, records); err != nil {
		return nil, err
	}
	return applied, nil
}

// Prune removes the tombstones stamped before the given stamp and says how many
// there were. A tombstone is only needed until every device has seen it.
func (vault *Vault) Prune(before string) (int, error) {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if err := vault.prepare(false); err != nil {
		return 0, err
	}
	if vault.key == nil {
		return 0, nil
	}
	plains, err := vault.openAll()
	if err != nil {
		return 0, err
	}
	records := make([]record, 0, len(plains))
	for index, plain := range plains {
		if !plain.Deleted || plain.Stamp >= before {
			records = append(records, vault.file.Records[index])
		}
	}
	pruned := len(plains) - len(records)
	if pruned == 0 {
		return 0, nil
	}
	if err := vault.write(vault.file.KDF, vault.file.WrappedKey, records); err != nil {
		return 0, err
	}
	return pruned, nil
}

// Names lists what is saved, sorted. It needs the vault unlocked.
func (vault *Vault) Names() ([]string, error) {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if err := vault.prepare(false); err != nil {
		return nil, err
	}
	var names []string
	if vault.key != nil {
		for _, entry := range vault.file.Records {
			plain, err := open(vault.key, entry)
			if err != nil {
				return nil, err
			}
			if !plain.Deleted {
				names = append(names, plain.Name)
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

// Clear forgets every saved secret and, with it, the master password and the
// data key: the next secret saved starts a vault of its own. It is the way out of
// ErrKeyLost, and of a forgotten master password.
func (vault *Vault) Clear() error {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	hadPassword := vault.file.KDF != nil
	if err := vault.write(nil, nil, nil); err != nil {
		return err
	}
	clear(vault.key)
	vault.key = nil
	if !hadPassword && vault.keys != nil {
		_ = vault.keys.Delete(KeyService, KeyAccount)
	}
	return nil
}

// SetMasterPassword protects the vault with a password, or changes it. The vault
// must be unlocked, except while it holds nothing. The data key leaves the
// system's store: opening the vault takes the password from then on.
func (vault *Vault) SetMasterPassword(password string) error {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if len(password) < MinMasterPasswordLength {
		return ErrWeakPassword
	}
	// An empty vault needs no store: its key is made here, and kept by the password.
	if err := vault.prepare(false); err != nil && !(errors.Is(err, ErrUnavailable) && len(vault.file.Records) == 0) {
		return err
	}
	if vault.key == nil { // empty, and no key yet
		vault.key = make([]byte, keySize)
		if _, err := rand.Read(vault.key); err != nil {
			vault.key = nil
			return err
		}
	}
	params := defaultKDF
	params.Salt = make([]byte, 16)
	if _, err := rand.Read(params.Salt); err != nil {
		return err
	}
	wrapped, err := wrapKey(vault.key, password, &params)
	if err != nil {
		return err
	}
	hadPassword := vault.file.KDF != nil
	if err := vault.write(&params, wrapped, vault.file.Records); err != nil {
		return err
	}
	if !hadPassword && vault.keys != nil {
		_ = vault.keys.Delete(KeyService, KeyAccount)
	}
	return nil
}

// RemoveMasterPassword puts the data key back in the system's store and drops the
// password. The vault must be unlocked. Without a store it refuses and keeps the
// password: nothing is ever left unprotected.
func (vault *Vault) RemoveMasterPassword() error {
	vault.mu.Lock()
	defer vault.mu.Unlock()
	if vault.file.KDF == nil {
		return nil
	}
	if vault.key == nil {
		return ErrLocked
	}
	if vault.keys == nil {
		return ErrUnavailable
	}
	if err := vault.keys.Store(KeyService, KeyAccount, vault.key); err != nil {
		if errors.Is(err, ErrKeystoreUnavailable) {
			return ErrUnavailable
		}
		return err
	}
	if err := vault.write(nil, nil, vault.file.Records); err != nil {
		_ = vault.keys.Delete(KeyService, KeyAccount)
		return err
	}
	return nil
}

// write saves a new state of the file, atomically, and adopts it on success.
func (vault *Vault) write(params *kdfParams, wrapped []byte, records []record) error {
	file := fileData{Version: fileVersion, KDF: params, WrappedKey: wrapped, Records: records}
	if file.Records == nil {
		file.Records = []record{}
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(vault.path, append(data, '\n')); err != nil {
		return err
	}
	vault.file = file
	return nil
}

func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".vault-*.tmp")
	if err != nil {
		return err
	}
	temporary := file.Name()
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		os.Remove(temporary)
		return err
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		os.Remove(temporary)
		return err
	}
	return nil
}

type plainRecord struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Stamp   string `json:"stamp,omitempty"`
	Deleted bool   `json:"deleted,omitempty"`
}

func recordID(key []byte, name string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(idContext))
	mac.Write([]byte(name))
	return mac.Sum(nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func seal(key []byte, plain plainRecord) (record, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return record{}, err
	}
	data, err := json.Marshal(plain)
	if err != nil {
		return record{}, err
	}
	id := recordID(key, plain.Name)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return record{}, err
	}
	return record{ID: id, Data: gcm.Seal(nonce, nonce, data, id)}, nil
}

func open(key []byte, entry record) (plainRecord, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return plainRecord{}, err
	}
	if len(entry.Data) < gcm.NonceSize() {
		return plainRecord{}, ErrCorrupt
	}
	nonce, sealed := entry.Data[:gcm.NonceSize()], entry.Data[gcm.NonceSize():]
	data, err := gcm.Open(nil, nonce, sealed, entry.ID)
	if err != nil {
		return plainRecord{}, ErrCorrupt
	}
	var plain plainRecord
	if err := json.Unmarshal(data, &plain); err != nil || !hmac.Equal(recordID(key, plain.Name), entry.ID) {
		return plainRecord{}, ErrCorrupt
	}
	return plain, nil
}

func deriveKey(password string, params *kdfParams) []byte {
	return argon2.IDKey([]byte(password), params.Salt, params.Time, params.MemoryKiB, params.Threads, keySize)
}

func wrapKey(key []byte, password string, params *kdfParams) ([]byte, error) {
	kek := deriveKey(password, params)
	defer clear(kek)
	gcm, err := newGCM(kek)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, key, []byte(wrapAssociated)), nil
}

func unwrapKey(file fileData, password string) ([]byte, error) {
	kek := deriveKey(password, file.KDF)
	defer clear(kek)
	gcm, err := newGCM(kek)
	if err != nil {
		return nil, err
	}
	if len(file.WrappedKey) < gcm.NonceSize() {
		return nil, ErrCorrupt
	}
	nonce, sealed := file.WrappedKey[:gcm.NonceSize()], file.WrappedKey[gcm.NonceSize():]
	key, err := gcm.Open(nil, nonce, sealed, []byte(wrapAssociated))
	if err != nil {
		return nil, ErrWrongPassword
	}
	if len(key) != keySize {
		return nil, ErrCorrupt
	}
	return key, nil
}

var shared struct {
	sync.Mutex
	vaults map[string]*Vault
}

// OpenShared is Open that hands every caller of the same path the same Vault, so
// the windows of one process, each saving into the one file, never overwrite each
// other's records with a stale copy of it.
func OpenShared(path string, keys Keystore) (*Vault, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	shared.Lock()
	defer shared.Unlock()
	if vault, ok := shared.vaults[absolute]; ok {
		return vault, nil
	}
	vault, err := Open(absolute, keys)
	if err != nil {
		return nil, err
	}
	if shared.vaults == nil {
		shared.vaults = make(map[string]*Vault)
	}
	shared.vaults[absolute] = vault
	return vault, nil
}

// Forget drops a shared Vault from the process's registry, so the next OpenShared
// reads the file again. A vault whose file was moved aside calls it.
func Forget(path string) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	shared.Lock()
	defer shared.Unlock()
	delete(shared.vaults, absolute)
}
