//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var procSHFileOperationW = syscall.NewLazyDLL("shell32.dll").NewProc("SHFileOperationW")

const (
	foDelete = 0x0003

	fofSilent          = 0x0004
	fofNoConfirmation  = 0x0010
	fofAllowUndo       = 0x0040
	fofNoErrorUI       = 0x0400
	fofWantNukeWarning = 0x4000
)

// shFileOpStruct mirrors SHFILEOPSTRUCTW (natural alignment on 64-bit).
type shFileOpStruct struct {
	hwnd                  uintptr
	wFunc                 uint32
	pFrom                 *uint16
	pTo                   *uint16
	fFlags                uint16
	fAnyOperationsAborted int32
	hNameMappings         uintptr
	lpszProgressTitle     *uint16
}

// trashPaths sends the paths to the Recycle Bin the way Explorer does, so
// Restore works. Items that can't be recycled (network shares, drives without
// a Recycle Bin) would otherwise be deleted for good; FOF_WANTNUKEWARNING makes
// the shell ask first, and declining leaves them reported as failed.
func trashPaths(paths []string) map[string]error {
	failed := map[string]error{}
	for _, p := range paths {
		if err := trashOne(p); err != nil {
			failed[p] = err
		}
	}
	return failed
}

func trashOne(p string) error {
	from, err := syscall.UTF16FromString(p)
	if err != nil {
		return err
	}
	from = append(from, 0) // pFrom is a double-null-terminated list
	op := shFileOpStruct{
		wFunc:  foDelete,
		pFrom:  &from[0],
		fFlags: fofAllowUndo | fofNoConfirmation | fofNoErrorUI | fofSilent | fofWantNukeWarning,
	}
	r, _, _ := procSHFileOperationW.Call(uintptr(unsafe.Pointer(&op)))
	if r != 0 {
		return fmt.Errorf("cannot move to Recycle Bin (error 0x%X)", r)
	}
	if op.fAnyOperationsAborted != 0 {
		return fmt.Errorf("moving to Recycle Bin was cancelled")
	}
	if _, err := os.Lstat(p); err == nil {
		return fmt.Errorf("cannot move to Recycle Bin")
	}
	return nil
}
