package knownhosts

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type renameFile func(oldPath, newPath string) error

func (file *File) delete(match func(Entry) bool, wanted int, rename renameFile) error {
	if wanted == 0 {
		return nil
	}
	file.mu.Lock()
	defer file.mu.Unlock()

	original, err := file.read()
	if err != nil {
		return err
	}
	lines, _ := parseDocument(original)
	removed := 0
	for index := range lines {
		if lines[index].hasEntry && match(lines[index].entry) {
			lines[index].remove = true
			removed++
		}
	}
	if removed != wanted {
		return ErrNotFound
	}

	replacement := make([]byte, 0, len(original))
	for _, line := range lines {
		if !line.remove {
			replacement = append(replacement, original[line.start:line.end]...)
		}
	}
	return replace(file.path, original, replacement, rename)
}

func replace(path string, original, replacement []byte, rename renameFile) (err error) {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".known_hosts-*")
	if err != nil {
		return fmt.Errorf("knownhosts: create temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		temporary.Close()
		if removeErr := os.Remove(temporaryPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && err == nil {
			err = fmt.Errorf("knownhosts: remove temporary file: %w", removeErr)
		}
	}()

	if err = temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("knownhosts: set temporary permissions: %w", err)
	}
	if _, err = temporary.Write(replacement); err != nil {
		return fmt.Errorf("knownhosts: write temporary file: %w", err)
	}
	if err = temporary.Sync(); err != nil {
		return fmt.Errorf("knownhosts: sync temporary file: %w", err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("knownhosts: close temporary file: %w", err)
	}

	current, readErr := os.ReadFile(path)
	if readErr != nil {
		return fmt.Errorf("knownhosts: verify source file: %w", readErr)
	}
	if !bytes.Equal(current, original) {
		return ErrChanged
	}
	if err = rename(temporaryPath, path); err != nil {
		return fmt.Errorf("knownhosts: replace file: %w", err)
	}
	return nil
}
