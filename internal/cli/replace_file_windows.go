//go:build windows

package cli

import (
	"os"
	"syscall"
	"unsafe"
)

const (
	moveFileReplaceExisting = 0x1
	moveFileWriteThrough    = 0x8
)

var moveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func replaceFile(temporaryName, output string) error {
	from, err := syscall.UTF16PtrFromString(temporaryName)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(output)
	if err != nil {
		return err
	}
	// Replace the destination in one native operation and wait for the move to
	// reach disk instead of deleting the existing file first.
	result, _, callErr := moveFileEx.Call(
		uintptr(unsafe.Pointer(from)),
		uintptr(unsafe.Pointer(to)),
		moveFileReplaceExisting|moveFileWriteThrough,
	)
	if result != 0 {
		return nil
	}
	if callErr == syscall.Errno(0) {
		callErr = syscall.EINVAL
	}
	return &os.LinkError{Op: "replace", Old: temporaryName, New: output, Err: callErr}
}
