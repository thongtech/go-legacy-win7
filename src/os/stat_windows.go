// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os

import (
	"errors"
	"internal/filepathlite"
	"internal/syscall/windows"
	"syscall"
	"unsafe"
)

// Stat returns the [FileInfo] structure describing file.
// If there is an error, it will be of type [*PathError].
func (file *File) Stat() (FileInfo, error) {
	if file == nil {
		return nil, ErrInvalid
	}
	return statHandle(file.name, file.pfd.Sysfd)
}

// stat implements both Stat and Lstat of a file.
func stat(funcname, name string, followSurrogates bool) (FileInfo, error) {
	if len(name) == 0 {
		return nil, &PathError{Op: funcname, Path: name, Err: syscall.Errno(syscall.ERROR_PATH_NOT_FOUND)}
	}
	namep, err := syscall.UTF16PtrFromString(fixLongPath(name))
	if err != nil {
		return nil, &PathError{Op: funcname, Path: name, Err: err}
	}

	// Try GetFileAttributesEx first, because it is faster than CreateFile.
	// See https://golang.org/issues/19922#issuecomment-300031421 for details.
	var fa syscall.Win32FileAttributeData
	err = syscall.GetFileAttributesEx(namep, syscall.GetFileExInfoStandard, (*byte)(unsafe.Pointer(&fa)))
	if errors.Is(err, ErrNotExist) && !isConsoleName(name) {
		return nil, &PathError{Op: "GetFileAttributesEx", Path: name, Err: err}
	}
	if err == nil && fa.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		// Not a surrogate for another named entity, because it isn't any kind of reparse point.
		// The information we got from GetFileAttributesEx is good enough for now.
		fs := newFileStatFromWin32FileAttributeData(&fa)
		if err := fs.saveInfoFromPath(name); err != nil {
			return nil, err
		}
		return fs, nil
	}

	// GetFileAttributesEx fails with ERROR_SHARING_VIOLATION error for
	// files like c:\pagefile.sys. Use FindFirstFile for such files.
	if err == windows.ERROR_SHARING_VIOLATION {
		var fd syscall.Win32finddata
		sh, err := syscall.FindFirstFile(namep, &fd)
		if err != nil {
			return nil, &PathError{Op: "FindFirstFile", Path: name, Err: err}
		}
		syscall.FindClose(sh)
		if fd.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
			// Not a surrogate for another named entity. FindFirstFile is good enough.
			fs := newFileStatFromWin32finddata(&fd)
			if err := fs.saveInfoFromPath(name); err != nil {
				return nil, err
			}
			return fs, nil
		}
	}

	// Use CreateFile to determine whether the file is a name surrogate and, if so,
	// save information about the link target.
	// Set FILE_FLAG_BACKUP_SEMANTICS so that CreateFile will create the handle
	// even if name refers to a directory.
	var flags uint32 = syscall.FILE_FLAG_BACKUP_SEMANTICS | syscall.FILE_FLAG_OPEN_REPARSE_POINT
	h, err := syscall.CreateFile(namep, 0, 0, nil, syscall.OPEN_EXISTING, flags, 0)

	// Windows 7 answers a console opened without access as missing, not
	// with ERROR_INVALID_PARAMETER.
	if err == windows.ERROR_INVALID_PARAMETER || (isConsoleName(name) && errors.Is(err, ErrNotExist)) {
		// Console handles, like "\\.\con", require generic read access. See
		// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew#consoles.
		// We haven't set it previously because it is normally not required
		// to read attributes and some files may not allow it.
		h, err = syscall.CreateFile(namep, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, flags, 0)
	}
	if bare, ok := consoleDeviceName(name); ok && errors.Is(err, ErrNotExist) {
		if barep, err2 := syscall.UTF16PtrFromString(bare); err2 == nil {
			h, err = syscall.CreateFile(barep, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, flags, 0)
		}
	}
	if err != nil {
		// Since CreateFile failed, we can't determine whether name refers to a
		// name surrogate, or some other kind of reparse point. Since we can't return a
		// FileInfo with a known-accurate Mode, we must return an error.
		return nil, &PathError{Op: "CreateFile", Path: name, Err: err}
	}

	fi, err := statHandle(name, h)
	var linkTarget string
	if err == nil && followSurrogates && followsMismatchedLinks() &&
		fi.(*fileStat).isReparseTagNameSurrogate() && fi.(*fileStat).FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 {
		// Read a file symlink's target through the handle in hand for
		// mismatchedLink.
		linkTarget, _ = readReparseLinkHandle(h)
	}
	syscall.CloseHandle(h)
	if err == nil && followSurrogates && fi.(*fileStat).isReparseTagNameSurrogate() {
		// To obtain information about the link target, we reopen the file without
		// FILE_FLAG_OPEN_REPARSE_POINT and examine the resulting handle.
		// (See https://devblogs.microsoft.com/oldnewthing/20100212-00/?p=14963.)
		h, err = syscall.CreateFile(namep, 0, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if err != nil {
			// name refers to a symlink, but we couldn't resolve the symlink target.
			return nil, &PathError{Op: "CreateFile", Path: name, Err: err}
		}
		defer syscall.CloseHandle(h)
		target, err := statHandle(name, h)
		if err == nil && followsMismatchedLinks() {
			if e := mismatchedLink(name, fi.(*fileStat), target.(*fileStat), linkTarget); e != nil {
				return nil, &PathError{Op: "CreateFile", Path: name, Err: e}
			}
		}
		return target, err
	}
	return fi, err
}

// followsMismatchedLinks is syscall's gate of the same name.
//
//go:linkname followsMismatchedLinks syscall.followsMismatchedLinks
func followsMismatchedLinks() bool

// mismatchedLink returns the refusal of a link to the other type when the
// link at name, with attributes link, is followed to target, or nil where
// the answer stands.
func mismatchedLink(name string, link, target *fileStat, linkTarget string) error {
	if link.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		if target.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 {
			return windows.ERROR_DIRECTORY
		}
		return nil
	}
	if target.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return syscall.ERROR_ACCESS_DENIED
	}
	// A chain ending at a file may start at a directory link, so check the
	// link's own target, read through its handle or else by name.
	t := linkTarget
	if t == "" {
		var err error
		if t, err = readlink(name); err != nil || t == "" {
			return nil
		}
	}
	if !filepathlite.IsAbs(t) && !IsPathSeparator(t[0]) {
		t = filepathlite.Dir(name) + `\` + t
	}
	// Only the target's own directory attribute matters.
	tp, err := syscall.UTF16PtrFromString(fixLongPath(t))
	if err != nil {
		return nil
	}
	var fa syscall.Win32FileAttributeData
	if syscall.GetFileAttributesEx(tp, syscall.GetFileExInfoStandard, (*byte)(unsafe.Pointer(&fa))) != nil {
		return nil
	}
	if fa.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		return syscall.ERROR_ACCESS_DENIED
	}
	return nil
}

// isConsoleName reports whether name is a console device, with or without
// the \\.\ prefix. Before Windows 8 GetFileAttributesEx reports them missing.
func isConsoleName(name string) bool {
	if len(name) >= 4 && IsPathSeparator(name[0]) && IsPathSeparator(name[1]) &&
		name[2] == '.' && IsPathSeparator(name[3]) {
		name = name[4:]
	}
	if len(name) < len("CON") || len(name) > len("CONOUT$") {
		return false
	}
	var upper [len("CONOUT$")]byte
	for i := 0; i < len(name); i++ {
		c := name[i]
		if 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		upper[i] = c
	}
	switch string(upper[:len(name)]) {
	case "CON", "CONIN$", "CONOUT$":
		return true
	}
	return false
}

// consoleDeviceName returns a \\.\ console name without the prefix. Before
// Windows 8 a prefixed name does not reach the console, so an open that comes
// back missing is retried bare.
func consoleDeviceName(name string) (string, bool) {
	if len(name) >= 4 && IsPathSeparator(name[0]) && IsPathSeparator(name[1]) &&
		name[2] == '.' && IsPathSeparator(name[3]) && isConsoleName(name) {
		return name[4:], true
	}
	return "", false
}

func statHandle(name string, h syscall.Handle) (FileInfo, error) {
	ft, err := syscall.GetFileType(h)
	if err != nil {
		return nil, &PathError{Op: "GetFileType", Path: name, Err: err}
	}
	switch ft {
	case syscall.FILE_TYPE_PIPE, syscall.FILE_TYPE_CHAR:
		return &fileStat{name: filepathlite.Base(name), filetype: ft}, nil
	}
	fs, err := newFileStatFromGetFileInformationByHandle(name, h)
	if err != nil {
		return nil, err
	}
	fs.filetype = ft
	return fs, nil
}

// statNolog implements Stat for Windows.
func statNolog(name string) (FileInfo, error) {
	return stat("Stat", name, true)
}

// lstatNolog implements Lstat for Windows.
func lstatNolog(name string) (FileInfo, error) {
	followSurrogates := false
	if name != "" && IsPathSeparator(name[len(name)-1]) {
		// We try to implement POSIX semantics for Lstat path resolution
		// (per https://pubs.opengroup.org/onlinepubs/9699919799.2013edition/basedefs/V1_chap04.html#tag_04_12):
		// symlinks before the last separator in the path must be resolved. Since
		// the last separator in this case follows the last path element, we should
		// follow symlinks in the last path element.
		followSurrogates = true
	}
	return stat("Lstat", name, followSurrogates)
}
