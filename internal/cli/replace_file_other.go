//go:build !windows

package cli

import "os"

func replaceDownloadedFile(temporaryName, output string) error {
	return os.Rename(temporaryName, output)
}
