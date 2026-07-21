//go:build !windows

package cli

import (
	"fmt"
	"os"
)

func protectCredentialPath(path string, directory bool) error {
	mode := os.FileMode(0o600)
	if directory {
		mode = 0o700
	}
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if permissions := info.Mode().Perm(); permissions != mode {
		return fmt.Errorf("permissions are %o, require %o", permissions, mode)
	}
	return nil
}

func verifyCredentialProtection(_ string, info os.FileInfo) error {
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		return fmt.Errorf("permissions are %o, require 600", permissions)
	}
	return nil
}
