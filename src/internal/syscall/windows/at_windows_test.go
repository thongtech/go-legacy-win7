// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package windows_test

import (
	"internal/syscall/windows"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestOpen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "a")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	tests := []struct {
		path string
		flag int
		err  error
	}{
		{dir, syscall.O_RDONLY, nil},
		{dir, syscall.O_CREAT, nil},
		{dir, syscall.O_RDONLY | syscall.O_CREAT, nil},
		{file, syscall.O_APPEND | syscall.O_WRONLY | os.O_CREATE, nil},
		{file, syscall.O_APPEND | syscall.O_WRONLY | os.O_CREATE | os.O_TRUNC, nil},
		{dir, syscall.O_RDONLY | syscall.O_TRUNC, syscall.ERROR_ACCESS_DENIED},
		{dir, syscall.O_WRONLY | syscall.O_RDWR, nil}, // TODO: syscall.Open returns EISDIR here, we should reconcile this
		{dir, syscall.O_WRONLY, syscall.EISDIR},
		{dir, syscall.O_RDWR, syscall.EISDIR},
	}
	for i, tt := range tests {
		dir := filepath.Dir(tt.path)
		dirfd, err := syscall.Open(dir, syscall.O_RDONLY, 0)
		if err != nil {
			t.Error(err)
			continue
		}
		base := filepath.Base(tt.path)
		h, err := windows.Openat(dirfd, base, uint64(tt.flag), 0o660)
		syscall.CloseHandle(dirfd)
		if err == nil {
			syscall.CloseHandle(h)
		}
		if err != tt.err {
			t.Errorf("%d: Open got %q, want %q", i, err, tt.err)
		}
	}
}

func TestDeleteAt(t *testing.T) {
	testCases := []struct {
		name     string
		modifier func(t *testing.T, path string)
	}{
		{"DeleteAt removes normal file", func(t *testing.T, name string) {}},
		{"DeleteAt removes file with no read permission", makeFileNotReadable},
		{"DeleteAt removes readonly file", makeFileReadonly},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			file := filepath.Join(dir, "a")
			f, err := os.Create(file)
			if err != nil {
				t.Fatal(err)
			}
			f.Close()

			// Remove all permissions from the file.
			// Do not use os.Chmod it sets only readonly attribute on Windows.
			tc.modifier(t, file)

			// delete file using DeleteAt
			dirfd, err := syscall.Open(dir, syscall.O_RDONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			base := filepath.Base(file)
			err = windows.Deleteat(dirfd, base, 0)
			syscall.CloseHandle(dirfd)
			if err != nil {
				t.Fatalf("Deleteat failed: %v", err)
			}

			// Verify the file has been deleted.
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				t.Fatalf("file still exists after DeleteAt")
			}
		})
	}
}

func makeFileReadonly(t *testing.T, name string) {
	if err := os.Chmod(name, 0); err != nil {
		t.Fatal(err)
	}
}

func makeFileNotReadable(t *testing.T, name string) {
	creatorOwnerSID, err := syscall.StringToSid("S-1-3-0")
	if err != nil {
		t.Fatal(err)
	}
	creatorGroupSID, err := syscall.StringToSid("S-1-3-1")
	if err != nil {
		t.Fatal(err)
	}
	everyoneSID, err := syscall.StringToSid("S-1-1-0")
	if err != nil {
		t.Fatal(err)
	}

	entryForSid := func(sid *syscall.SID) windows.EXPLICIT_ACCESS {
		return windows.EXPLICIT_ACCESS{
			AccessPermissions: 0,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee: windows.TRUSTEE{
				TrusteeForm: windows.TRUSTEE_IS_SID,
				Name:        (uintptr)(unsafe.Pointer(sid)),
			},
		}
	}

	entries := []windows.EXPLICIT_ACCESS{
		entryForSid(creatorOwnerSID),
		entryForSid(creatorGroupSID),
		entryForSid(everyoneSID),
	}

	var oldAcl, newAcl *windows.ACL
	if err := windows.SetEntriesInAcl(
		uint32(len(entries)),
		&entries[0],
		oldAcl,
		&newAcl,
	); err != nil {
		t.Fatal(err)
	}

	defer syscall.LocalFree((syscall.Handle)(unsafe.Pointer(newAcl)))
	if err := windows.SetNamedSecurityInfo(
		name,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		newAcl,
		nil,
	); err != nil {
		t.Fatal(err)
	}
}

// TestRenameToKeepsExisting checks that renameTo replaces a name only when
// told to, which the way back from a rename aside relies on.
func TestRenameToKeepsExisting(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	open := func(name string, access uint32, flags uint32) syscall.Handle {
		p, err := syscall.UTF16PtrFromString(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		h, err := syscall.CreateFile(p, access,
			syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
			nil, syscall.OPEN_EXISTING, flags, 0)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	dh := open(".", syscall.GENERIC_READ, syscall.FILE_FLAG_BACKUP_SEMANTICS)
	defer syscall.CloseHandle(dh)
	// h holds DELETE access, which a reader that does not share deletion
	// cannot coexist with, so it is closed before the file is read back.
	h := open("a", windows.DELETE|windows.SYNCHRONIZE, 0)
	closed := false
	defer func() {
		if !closed {
			syscall.CloseHandle(h)
		}
	}()
	b16, err := syscall.UTF16FromString("b")
	if err != nil {
		t.Fatal(err)
	}
	b16 = b16[:len(b16)-1]

	if err := windows.RenameTo(h, dh, b16, false); err == nil {
		t.Fatal("renameTo replaced b without being asked to")
	}
	if data, err := os.ReadFile(filepath.Join(dir, "b")); err != nil || string(data) != "b" {
		t.Fatalf("after the refused rename, b holds %q, %v; want %q", data, err, "b")
	}
	if err := windows.RenameTo(h, dh, b16, true); err != nil {
		t.Fatal(err)
	}
	syscall.CloseHandle(h)
	closed = true
	if data, err := os.ReadFile(filepath.Join(dir, "b")); err != nil || string(data) != "a" {
		t.Fatalf("after the rename, b holds %q, %v; want %q", data, err, "a")
	}
}

// TestOnDiskName checks that the delete fallback takes names on a disk, in
// every form, and none in the pipe, mailslot or device namespaces.
func TestOnDiskName(t *testing.T) {
	for _, tt := range []struct {
		name string
		want bool
	}{
		{`C:\dir\file`, true},
		{`c:/dir/file`, true},
		{`relative\file`, true},
		{`\\?\C:\dir\file`, true},
		{`\\.\C:\dir\file`, true},
		{`\\server\share\dir\file`, true},
		{`\\?\UNC\server\share\file`, true},
		{`\\?\Volume{01234567-89ab-cdef-0123-456789abcdef}\file`, true},
		{`\\.\pipe\name`, false},
		{`\\?\pipe\name`, false},
		{`\\server\pipe\name`, false},
		{`\\server\PIPE\name`, false},
		{`\\.\mailslot\name`, false},
		{`\\server\mailslot\name`, false},
		{`\\server\pipe`, false},
		{`\\server\MailSlot`, false},
		{`\\server\pipes\file`, true},
		{`NUL`, false},
		// A device name inside a directory follows the kernel's own
		// conversion, which Windows 11 no longer maps to the device, so
		// only the bare name is asked.
		{`COM1`, false},
		{`\\.\COM1`, false},
		{`\\server`, false},
	} {
		if got := windows.OnDiskName(tt.name); got != tt.want {
			t.Errorf("onDiskName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestDeleteatAnswersPendingAtOnce checks that a kernel with POSIX delete
// answers a name pending deletion at once, as upstream, without the wait.
func TestDeleteatAnswersPendingAtOnce(t *testing.T) {
	// The hook takes the wait on any kernel.
	if !windows.KernelDeletesAsUpstream() || windows.TestDeleteatFallback {
		t.Skip("the wait is for this kernel")
	}
	dir := t.TempDir()
	name := filepath.Join(dir, "f")
	if err := os.WriteFile(name, nil, 0666); err != nil {
		t.Fatal(err)
	}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(p, windows.DELETE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil,
		syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(h)
	// The old disposition class keeps the name until h closes.
	mark := byte(1)
	if err := windows.SetFileInformationByHandle(h, windows.FileDispositionInfo, unsafe.Pointer(&mark), 1); err != nil {
		t.Fatal(err)
	}
	dp, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		t.Fatal(err)
	}
	d, err := syscall.CreateFile(dp, syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.CloseHandle(d)
	start := time.Now()
	err = windows.Deleteat(d, "f", 0)
	if elapsed := time.Since(start); elapsed >= 50*time.Millisecond {
		t.Errorf("Deleteat of a name marked for deletion took %v, want an answer at once", elapsed)
	}
	if err == nil {
		t.Error("Deleteat of a name marked for deletion = nil, want the refusal upstream gives")
	}
}
