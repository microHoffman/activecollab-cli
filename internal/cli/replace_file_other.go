//go:build !windows

package cli

import "os"

func replaceFile(temporaryName, output string) error {
	return os.Rename(temporaryName, output)
}
