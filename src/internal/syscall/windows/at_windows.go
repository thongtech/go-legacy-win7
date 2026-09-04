// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package windows

import (
	"internal/oserror"
	"runtime"
	"structs"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"
)

// Openat flags supported by syscall.Open.
const (
	O_DIRECTORY = 0x04000 // target must be a directory
)

// Openat flags not supported by syscall.Open.
//
// These are invented values, use values in the 33-63 bit range
// to avoid overlap with flags and attributes supported by [syscall.Open].
//
// When adding a new flag here, add an unexported version to
// the set of invented O_ values in syscall/types_windows.go
// to avoid overlap.
const (
	O_NOFOLLOW_ANY = 0x200000000 // disallow symlinks anywhere in the path
	O_WRITE_ATTRS  = 0x800000000 // FILE_WRITE_ATTRIBUTES, used by Chmod
)

// objDontReparseUnsupported records that this kernel rejected
// OBJ_DONT_REPARSE, which Windows 7 does. The retry opens with
// FILE_OPEN_REPARSE_POINT instead, so a reparse point is returned rather than
// followed and Openat reports STATUS_REPARSE_POINT_ENCOUNTERED itself.
var objDontReparseUnsupported atomic.Bool

// isReparsePoint reports whether h refers to a reparse point.
func isReparsePoint(h syscall.Handle) (bool, error) {
	var d syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &d); err != nil {
		return false, err
	}
	return d.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}

// TestOpenatNoObjDontReparse should only be used for testing purposes.
// When set, [Openat] behaves as it does on a kernel without
// OBJ_DONT_REPARSE.
var TestOpenatNoObjDontReparse bool

// ObjDontReparseUnsupportedForTest should only be used for testing purposes.
// It reports whether [Openat] has concluded that this kernel rejects
// OBJ_DONT_REPARSE.
func ObjDontReparseUnsupportedForTest() bool {
	return objDontReparseUnsupported.Load()
}

func Openat(dirfd syscall.Handle, name string, flag uint64, perm uint32) (_ syscall.Handle, e1 error) {
	if len(name) == 0 {
		return syscall.InvalidHandle, syscall.ERROR_FILE_NOT_FOUND
	}

	var access, options uint32
	// Map Win32 file flags to NT create options.
	fileFlags := uint32(flag) & FileFlagsMask
	if fileFlags&^ValidFileFlagsMask != 0 {
		return syscall.InvalidHandle, oserror.ErrInvalid
	}
	if fileFlags&O_FILE_FLAG_OVERLAPPED == 0 {
		options |= FILE_SYNCHRONOUS_IO_NONALERT
	}
	if fileFlags&O_FILE_FLAG_DELETE_ON_CLOSE != 0 {
		access |= DELETE
	}
	setOptionFlag := func(ntFlag, win32Flag uint32) {
		if fileFlags&win32Flag != 0 {
			options |= ntFlag
		}
	}
	setOptionFlag(FILE_NO_INTERMEDIATE_BUFFERING, O_FILE_FLAG_NO_BUFFERING)
	setOptionFlag(FILE_WRITE_THROUGH, O_FILE_FLAG_WRITE_THROUGH)
	setOptionFlag(FILE_SEQUENTIAL_ONLY, O_FILE_FLAG_SEQUENTIAL_SCAN)
	setOptionFlag(FILE_RANDOM_ACCESS, O_FILE_FLAG_RANDOM_ACCESS)
	setOptionFlag(FILE_OPEN_FOR_BACKUP_INTENT, O_FILE_FLAG_BACKUP_SEMANTICS)
	setOptionFlag(FILE_SESSION_AWARE, O_FILE_FLAG_SESSION_AWARE)
	setOptionFlag(FILE_DELETE_ON_CLOSE, O_FILE_FLAG_DELETE_ON_CLOSE)
	setOptionFlag(FILE_OPEN_NO_RECALL, O_FILE_FLAG_OPEN_NO_RECALL)
	setOptionFlag(FILE_OPEN_REPARSE_POINT, O_FILE_FLAG_OPEN_REPARSE_POINT)

	switch flag & (syscall.O_RDONLY | syscall.O_WRONLY | syscall.O_RDWR) {
	case syscall.O_RDONLY:
		// FILE_GENERIC_READ includes FILE_LIST_DIRECTORY.
		access |= FILE_GENERIC_READ
	case syscall.O_WRONLY:
		access |= FILE_GENERIC_WRITE
		options |= FILE_NON_DIRECTORY_FILE
	case syscall.O_RDWR:
		access |= FILE_GENERIC_READ | FILE_GENERIC_WRITE
		options |= FILE_NON_DIRECTORY_FILE
	default:
		// Stat opens files without requesting read or write permissions,
		// but we still need to request SYNCHRONIZE.
		access |= SYNCHRONIZE
	}
	if flag&syscall.O_CREAT != 0 {
		access |= FILE_GENERIC_WRITE
	}
	if fileFlags&O_FILE_FLAG_NO_BUFFERING != 0 {
		// Disable buffering implies no implicit append access.
		access &^= FILE_APPEND_DATA
	}
	if flag&syscall.O_APPEND != 0 {
		access |= FILE_APPEND_DATA
		// Remove FILE_WRITE_DATA access unless O_TRUNC is set,
		// in which case we need it to truncate the file.
		if flag&syscall.O_TRUNC == 0 {
			access &^= FILE_WRITE_DATA
		}
	}
	if flag&O_DIRECTORY != 0 {
		options |= FILE_DIRECTORY_FILE
		access |= FILE_LIST_DIRECTORY
	}
	if flag&syscall.O_SYNC != 0 {
		options |= FILE_WRITE_THROUGH
	}
	if flag&O_WRITE_ATTRS != 0 {
		access |= FILE_WRITE_ATTRIBUTES
	}
	// Allow File.Stat.
	access |= STANDARD_RIGHTS_READ | FILE_READ_ATTRIBUTES | FILE_READ_EA

	objAttrs := &OBJECT_ATTRIBUTES{}
	noFollow := flag&O_NOFOLLOW_ANY != 0
	emulateNoFollow := noFollow && (objDontReparseUnsupported.Load() || TestOpenatNoObjDontReparse)
	if noFollow && !emulateNoFollow {
		objAttrs.Attributes |= OBJ_DONT_REPARSE
	}
	if emulateNoFollow {
		// Emulate OBJ_DONT_REPARSE. See objDontReparseUnsupported.
		options |= FILE_OPEN_REPARSE_POINT
	}
	if flag&syscall.O_CLOEXEC == 0 {
		objAttrs.Attributes |= OBJ_INHERIT
	}
	if fileFlags&O_FILE_FLAG_POSIX_SEMANTICS == 0 {
		objAttrs.Attributes |= OBJ_CASE_INSENSITIVE
	}
	if err := objAttrs.init(dirfd, name); err != nil {
		return syscall.InvalidHandle, err
	}

	// We don't use FILE_OVERWRITE/FILE_OVERWRITE_IF, because when opening
	// a file with FILE_ATTRIBUTE_READONLY these will replace an existing
	// file with a new, read-only one.
	//
	// Instead, we ftruncate the file after opening when O_TRUNC is set.
	var disposition uint32
	switch {
	case flag&(syscall.O_CREAT|syscall.O_EXCL) == (syscall.O_CREAT | syscall.O_EXCL):
		disposition = FILE_CREATE
		options |= FILE_OPEN_REPARSE_POINT // don't follow symlinks
	case flag&syscall.O_CREAT == syscall.O_CREAT:
		disposition = FILE_OPEN_IF
	default:
		disposition = FILE_OPEN
	}

	fileAttrs := uint32(FILE_ATTRIBUTE_NORMAL)
	if perm&syscall.S_IWRITE == 0 {
		fileAttrs = FILE_ATTRIBUTE_READONLY
	}

	var h syscall.Handle
	create := func() error {
		return NtCreateFile(
			&h,
			SYNCHRONIZE|access,
			objAttrs,
			&IO_STATUS_BLOCK{},
			nil,
			fileAttrs,
			FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE,
			disposition,
			FILE_OPEN_FOR_BACKUP_INTENT|options,
			nil,
			0,
		)
	}
	err := create()
	if err == STATUS_INVALID_PARAMETER && objAttrs.Attributes&OBJ_DONT_REPARSE != 0 {
		// STATUS_INVALID_PARAMETER has two causes here. One is a kernel
		// that does not know OBJ_DONT_REPARSE. The other is an open asking
		// for a directory and a non-directory at once, which a trailing
		// separator on a file does. Dropping the attribute cannot fix the
		// second, so retry without it and take the kernel to lack it only
		// if STATUS_INVALID_PARAMETER goes away.
		objAttrs.Attributes &^= OBJ_DONT_REPARSE
		options |= FILE_OPEN_REPARSE_POINT
		if retry := create(); retry != STATUS_INVALID_PARAMETER {
			objDontReparseUnsupported.Store(true)
			emulateNoFollow = true
			err = retry
		}
	}
	if err != nil {
		return h, ntCreateFileError(err, flag)
	}
	if emulateNoFollow && fileFlags&O_FILE_FLAG_OPEN_REPARSE_POINT == 0 {
		// The emulated OBJ_DONT_REPARSE refusal.
		isLink, err := isReparsePoint(h)
		if err != nil {
			syscall.CloseHandle(h)
			return syscall.InvalidHandle, err
		}
		if isLink {
			syscall.CloseHandle(h)
			return syscall.InvalidHandle, ntCreateFileError(STATUS_REPARSE_POINT_ENCOUNTERED, flag)
		}
	}

	if flag&syscall.O_TRUNC != 0 {
		err = syscall.Ftruncate(h, 0)
		if err == ERROR_INVALID_PARAMETER {
			// ERROR_INVALID_PARAMETER means truncation is not supported on this file handle.
			// Unix's O_TRUNC specification says to ignore O_TRUNC on named pipes and terminal devices.
			// We do the same here.
			if t, err1 := syscall.GetFileType(h); err1 == nil && (t == syscall.FILE_TYPE_PIPE || t == syscall.FILE_TYPE_CHAR) {
				err = nil
			}
		}
		if err != nil {
			syscall.CloseHandle(h)
			return syscall.InvalidHandle, err
		}
	}

	return h, nil
}

// ntCreateFileError maps error returns from NTCreateFile to user-visible errors.
func ntCreateFileError(err error, flag uint64) error {
	s, ok := err.(NTStatus)
	if !ok {
		// Shouldn't really be possible, NtCreateFile always returns NTStatus.
		return err
	}
	switch s {
	case STATUS_REPARSE_POINT_ENCOUNTERED:
		return syscall.ELOOP
	case STATUS_NOT_A_DIRECTORY:
		// ENOTDIR is the errno returned by open when O_DIRECTORY is specified
		// and the target is not a directory.
		//
		// NtCreateFile can return STATUS_NOT_A_DIRECTORY under other circumstances,
		// such as when opening "file/" where "file" is not a directory.
		// (This might be Windows version dependent.)
		//
		// Only map STATUS_NOT_A_DIRECTORY to ENOTDIR when O_DIRECTORY is specified.
		if flag&O_DIRECTORY != 0 {
			return syscall.ENOTDIR
		}
	case STATUS_FILE_IS_A_DIRECTORY:
		return syscall.EISDIR
	case STATUS_OBJECT_NAME_COLLISION:
		return syscall.EEXIST
	}
	return s.Errno()
}

func Mkdirat(dirfd syscall.Handle, name string, mode uint32) error {
	objAttrs := &OBJECT_ATTRIBUTES{}
	if err := objAttrs.init(dirfd, name); err != nil {
		return err
	}
	var h syscall.Handle
	err := NtCreateFile(
		&h,
		FILE_GENERIC_READ,
		objAttrs,
		&IO_STATUS_BLOCK{},
		nil,
		syscall.FILE_ATTRIBUTE_NORMAL,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		FILE_CREATE,
		FILE_DIRECTORY_FILE,
		nil,
		0,
	)
	if err != nil {
		return ntCreateFileError(err, 0)
	}
	syscall.CloseHandle(h)
	return nil
}

func Deleteat(dirfd syscall.Handle, name string, options uint32) error {
	if name == "." {
		// NtOpenFile's documentation isn't explicit about what happens when deleting ".".
		// Make this an error consistent with that of POSIX.
		return syscall.EINVAL
	}
	objAttrs := &OBJECT_ATTRIBUTES{}
	if err := objAttrs.init(dirfd, name); err != nil {
		return err
	}
	var h syscall.Handle
	err := NtOpenFile(
		&h,
		FILE_READ_ATTRIBUTES|DELETE,
		objAttrs,
		&IO_STATUS_BLOCK{},
		FILE_SHARE_DELETE|FILE_SHARE_READ|FILE_SHARE_WRITE,
		FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT|options,
	)
	if err != nil {
		if ntStatus, ok := err.(NTStatus); !ok || ntStatus != STATUS_ACCESS_DENIED {
			return ntCreateFileError(err, 0)
		}

		// Access denied, try opening with DELETE only.
		// This may succeed if the file has restrictive permissions
		// but the caller has delete child permission on the parent directory.
		err = NtOpenFile(
			&h,
			DELETE,
			objAttrs,
			&IO_STATUS_BLOCK{},
			FILE_SHARE_DELETE|FILE_SHARE_READ|FILE_SHARE_WRITE,
			FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT|options,
		)
		if err != nil {
			return ntCreateFileError(err, 0)
		}
	}
	defer syscall.CloseHandle(h)

	if TestDeleteatFallback {
		return deleteatFallback(h, objAttrs, name)
	}

	const FileDispositionInformationEx = 64

	// First, attempt to delete the file using POSIX semantics
	// (which permit a file to be deleted while it is still open).
	// This matches the behavior of DeleteFileW.
	//
	// The following call uses features available on different Windows versions:
	// - FILE_DISPOSITION_INFORMATION_EX: Windows 10, version 1607 (aka RS1)
	// - FILE_DISPOSITION_POSIX_SEMANTICS: Windows 10, version 1607 (aka RS1)
	// - FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE: Windows 10, version 1809 (aka RS5)
	//
	// Also, some file systems, like FAT32, don't support POSIX semantics.
	err = NtSetInformationFile(
		h,
		&IO_STATUS_BLOCK{},
		unsafe.Pointer(&FILE_DISPOSITION_INFORMATION_EX{
			Flags: FILE_DISPOSITION_DELETE |
				FILE_DISPOSITION_FORCE_IMAGE_SECTION_CHECK |
				FILE_DISPOSITION_POSIX_SEMANTICS |
				// This differs from DeleteFileW, but matches os.Remove's
				// behavior on Unix platforms of permitting deletion of
				// read-only files.
				FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE,
		}),
		uint32(unsafe.Sizeof(FILE_DISPOSITION_INFORMATION_EX{})),
		FileDispositionInformationEx,
	)
	switch err {
	case nil:
		return nil
	case STATUS_INVALID_INFO_CLASS, // the operating system doesn't support FileDispositionInformationEx
		STATUS_INVALID_PARAMETER, // the operating system doesn't support one of the flags
		STATUS_NOT_SUPPORTED:     // the file system doesn't support FILE_DISPOSITION_INFORMATION_EX or one of the flags
		return deleteatFallback(h, objAttrs, name)
	default:
		return err.(NTStatus).Errno()
	}
}

// TestDeleteatFallback should only be used for testing purposes.
// When set, [Deleteat] uses the fallback path unconditionally.
var TestDeleteatFallback bool

// TestDeleteatBeforeReopen should only be used for testing purposes. When
// set, the fallback calls it before opening the name again to clear the
// read-only attribute, so that a test can give the name to another file.
var TestDeleteatBeforeReopen func()

// deleteatFallback is a deleteat implementation that strives
// for compatibility with older Windows versions and file systems
// over performance.
func deleteatFallback(h syscall.Handle, objAttrs *OBJECT_ATTRIBUTES, name string) error {
	var data syscall.ByHandleFileInformation
	haveData := syscall.GetFileInformationByHandle(h, &data) == nil
	if haveData && data.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		// Remove read-only attribute. Open the name again, as it was previously open without FILE_WRITE_ATTRIBUTES
		// access in order to maximize compatibility in the happy path. ReOpenFile would serve for a file, but it
		// refuses a directory with ERROR_ACCESS_DENIED, and a read-only directory has to be cleared like any other.
		if TestDeleteatBeforeReopen != nil {
			TestDeleteatBeforeReopen()
		}
		var wh syscall.Handle
		err := NtOpenFile(
			&wh,
			SYNCHRONIZE|FILE_READ_ATTRIBUTES|FILE_WRITE_ATTRIBUTES,
			objAttrs,
			&IO_STATUS_BLOCK{},
			FILE_SHARE_DELETE|FILE_SHARE_READ|FILE_SHARE_WRITE,
			FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT|FILE_SYNCHRONOUS_IO_NONALERT,
		)
		if err != nil {
			return ntCreateFileError(err, 0)
		}
		// The name may have changed hands since h was opened. Clear the attribute only
		// on the entry h holds, and otherwise go on with the attribute in place.
		var again syscall.ByHandleFileInformation
		if syscall.GetFileInformationByHandle(wh, &again) == nil &&
			again.VolumeSerialNumber == data.VolumeSerialNumber &&
			again.FileIndexHigh == data.FileIndexHigh && again.FileIndexLow == data.FileIndexLow {
			err = SetFileInformationByHandle(
				wh,
				FileBasicInfo,
				unsafe.Pointer(&FILE_BASIC_INFO{
					FileAttributes: data.FileAttributes &^ FILE_ATTRIBUTE_READONLY,
				}),
				uint32(unsafe.Sizeof(FILE_BASIC_INFO{})),
			)
		}
		syscall.CloseHandle(wh)
		if err != nil {
			return err
		}
	}

	// Without POSIX semantics the name lives on until the last handle to the
	// file closes. A caller still holding the file open would go on seeing
	// the name, and the directory holding it could not be removed. Move the
	// file aside first, so that the name goes now and the file itself goes
	// when the last handle does. Leave directories where they are, since
	// deleting one has to fail while it still has entries and moving it
	// first would scatter the tree.
	aside := haveData &&
		data.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 &&
		setAsideForDelete(h, objAttrs, &data)

	err := SetFileInformationByHandle(
		h,
		FileDispositionInfo,
		unsafe.Pointer(&FILE_DISPOSITION_INFO{
			DeleteFile: true,
		}),
		uint32(unsafe.Sizeof(FILE_DISPOSITION_INFO{})),
	)
	if err != nil && aside {
		// The file is staying. Put the name back unless something else has
		// taken it since. The file is better off a stray in the staging
		// directory than in the place of that.
		if p16, err := syscall.UTF16FromString(name); err == nil {
			renameTo(h, objAttrs.RootDirectory, p16[:len(p16)-1], false)
		}
	}
	return err
}

// deleteStagingDir is a directory to park a file that cannot go away at once,
// with the volume it sits on. Parking a file there takes it out of the tree a
// caller is removing.
type deleteStagingDir struct {
	h      syscall.Handle
	volume uint32
	ok     bool
}

var deleteStaging = sync.OnceValue(func() (s deleteStagingDir) {
	var buf []uint16
	n := uint32(syscall.MAX_PATH)
	for {
		buf = make([]uint16, n)
		var err error
		n, err = syscall.GetTempPath(uint32(len(buf)), &buf[0])
		if err != nil || n == 0 {
			return
		}
		if n <= uint32(len(buf)) {
			break
		}
	}
	h, err := syscall.CreateFile(
		&buf[0],
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return
	}
	var data syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &data); err != nil {
		syscall.CloseHandle(h)
		return
	}
	return deleteStagingDir{h: h, volume: data.VolumeSerialNumber, ok: true}
})

// setAsideForDelete moves the file open on h off the name it was reached by,
// preferring the staging directory so that the file leaves the tree entirely.
// A rename cannot cross volumes, and it needs every other handle to the file
// to share deletion, so this reports whether it happened.
func setAsideForDelete(h syscall.Handle, objAttrs *OBJECT_ATTRIBUTES, data *syscall.ByHandleFileInformation) bool {
	dir := objAttrs.RootDirectory
	if s := deleteStaging(); s.ok && s.volume == data.VolumeSerialNumber {
		dir = s.h
	}
	if dir == 0 {
		return false
	}
	// The volume already keeps an identifier unique to the file, so two
	// files set aside at the same time cannot land on the same name.
	const hex = "0123456789abcdef"
	name := make([]uint16, 0, len(".go-deleted-")+16)
	for _, c := range ".go-deleted-" {
		name = append(name, uint16(c))
	}
	for _, v := range [2]uint32{data.FileIndexHigh, data.FileIndexLow} {
		for shift := 28; shift >= 0; shift -= 4 {
			name = append(name, uint16(hex[(v>>uint(shift))&0xf]))
		}
	}
	return renameTo(h, dir, name, true) == nil
}

// renameTo renames the file open on h to name within dir, which has to be on
// the volume the file is already on. Whatever holds the name already is
// replaced only if replace is set.
func renameTo(h syscall.Handle, dir syscall.Handle, name []uint16, replace bool) error {
	info := FILE_RENAME_INFORMATION{
		RootDirectory: dir,
	}
	if replace {
		info.ReplaceIfExists = true
	}
	if len(name) > len(info.FileName) {
		return syscall.EINVAL
	}
	copy(info.FileName[:], name)
	info.FileNameLength = uint32(len(name) * 2)
	const FileRenameInformation = 10
	return NtSetInformationFile(
		h,
		&IO_STATUS_BLOCK{},
		unsafe.Pointer(&info),
		uint32(unsafe.Sizeof(info)),
		FileRenameInformation,
	)
}

func Renameat(olddirfd syscall.Handle, oldpath string, newdirfd syscall.Handle, newpath string) error {
	objAttrs := &OBJECT_ATTRIBUTES{}
	if err := objAttrs.init(olddirfd, oldpath); err != nil {
		return err
	}
	var h syscall.Handle
	open := func(access uint32) error {
		return NtOpenFile(
			&h,
			access,
			objAttrs,
			&IO_STATUS_BLOCK{},
			FILE_SHARE_DELETE|FILE_SHARE_READ|FILE_SHARE_WRITE,
			FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT|FILE_SYNCHRONOUS_IO_NONALERT,
		)
	}
	// FILE_READ_ATTRIBUTES is for isDirectory. A file can grant deletion
	// without it, so then ask for what the rename needs alone.
	err := open(SYNCHRONIZE | DELETE | FILE_READ_ATTRIBUTES)
	if err == STATUS_ACCESS_DENIED {
		err = open(SYNCHRONIZE | DELETE)
	}
	if err != nil {
		return ntCreateFileError(err, 0)
	}
	defer syscall.CloseHandle(h)

	renameInfoEx := FILE_RENAME_INFORMATION_EX{
		Flags: FILE_RENAME_REPLACE_IF_EXISTS |
			FILE_RENAME_POSIX_SEMANTICS,
		RootDirectory: newdirfd,
	}
	p16, err := syscall.UTF16FromString(newpath)
	if err != nil {
		return err
	}
	if len(p16) > len(renameInfoEx.FileName) {
		return syscall.EINVAL
	}
	copy(renameInfoEx.FileName[:], p16)
	renameInfoEx.FileNameLength = uint32((len(p16) - 1) * 2)

	const (
		FileRenameInformation   = 10
		FileRenameInformationEx = 65
	)
	if !TestRenameatNoPosixSemantics {
		err = NtSetInformationFile(
			h,
			&IO_STATUS_BLOCK{},
			unsafe.Pointer(&renameInfoEx),
			uint32(unsafe.Sizeof(FILE_RENAME_INFORMATION_EX{})),
			FileRenameInformationEx,
		)
		if err == nil {
			return nil
		}
	}
	// A kernel or a file system without POSIX rename semantics answers with one
	// of these. Any other answer is a refusal by one that has them, which the
	// fallback below leaves standing, so that a program sees the same result as
	// with an unpatched toolchain.
	noPosixSemantics := TestRenameatNoPosixSemantics ||
		err == STATUS_INVALID_INFO_CLASS || err == STATUS_INVALID_PARAMETER || err == STATUS_NOT_SUPPORTED

	// If the prior rename failed, the filesystem might not support
	// POSIX semantics (for example, FAT), or might not have implemented
	// FILE_RENAME_INFORMATION_EX.
	//
	// Try again.
	renameInfo := FILE_RENAME_INFORMATION{
		ReplaceIfExists: true,
		RootDirectory:   newdirfd,
	}
	copy(renameInfo.FileName[:], p16)
	renameInfo.FileNameLength = renameInfoEx.FileNameLength

	renameFile := func() error {
		return NtSetInformationFile(
			h,
			&IO_STATUS_BLOCK{},
			unsafe.Pointer(&renameInfo),
			uint32(unsafe.Sizeof(FILE_RENAME_INFORMATION{})),
			FileRenameInformation,
		)
	}
	err = renameFile()
	if err == STATUS_ACCESS_DENIED && noPosixSemantics && !isDirectory(h) {
		// Without POSIX semantics the rename cannot replace a link, which counts as a
		// directory, or a file another handle holds open. A rename with POSIX semantics
		// replaces both. Move the destination aside and rename again. The moved copy is
		// deleted once the rename has succeeded and moved back if it has not, so that a
		// crash between the steps leaves an extra file rather than a missing one.
		//
		// A directory source is left to the plain rename. With POSIX semantics a
		// directory replaces an idle file, which the plain rename does as well, and
		// nothing else.
		if replaced, rerr := replaceRenameTarget(newdirfd, newpath, renameFile); replaced {
			err = rerr
		}
	}
	if st, ok := err.(NTStatus); ok {
		return st.Errno()
	}
	return err
}

// isDirectory reports whether h is open on a directory itself, rather than on
// a file or on a link that stands in for a directory. When it cannot tell it
// says yes, which only ever leaves a rename target in place.
func isDirectory(h syscall.Handle) bool {
	var d syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &d); err != nil {
		return true
	}
	const dirOrLink = syscall.FILE_ATTRIBUTE_DIRECTORY | syscall.FILE_ATTRIBUTE_REPARSE_POINT
	return d.FileAttributes&dirOrLink == syscall.FILE_ATTRIBUTE_DIRECTORY
}

// TestRenameatNoPosixSemantics should only be used for testing purposes.
// When set, [Renameat] behaves as it does on a kernel without
// FileRenameInformationEx.
var TestRenameatNoPosixSemantics bool

// TestRenameatRetryFails should only be used for testing purposes. When set,
// the rename [Renameat] retries after moving the destination aside fails, as
// it would if the source had become unrenamable in between.
var TestRenameatRetryFails bool

// nameSurrogateBit is set in the reparse tag of a reparse point that stands in
// for another named entity, a symlink or a mount point, rather than one that
// merely carries data for a filter driver.
const nameSurrogateBit = 0x20000000

// replaceRenameTarget stands in for a rename with POSIX semantics where the
// kernel has none. It moves the destination aside, calls rename again, and
// deletes the moved copy once that has succeeded, or moves it back when it has
// not. It leaves alone what a rename with POSIX semantics refuses, a directory
// that is not a link and a read-only file. It reports whether the second
// rename was reached, and that rename's result.
func replaceRenameTarget(dirfd syscall.Handle, name string, rename func() error) (bool, error) {
	objAttrs := &OBJECT_ATTRIBUTES{}
	if err := objAttrs.init(dirfd, name); err != nil {
		return false, nil
	}
	var th syscall.Handle
	err := NtOpenFile(
		&th,
		SYNCHRONIZE|DELETE|FILE_READ_ATTRIBUTES,
		objAttrs,
		&IO_STATUS_BLOCK{},
		FILE_SHARE_DELETE|FILE_SHARE_READ|FILE_SHARE_WRITE,
		FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT|FILE_SYNCHRONOUS_IO_NONALERT,
	)
	if err != nil {
		return false, nil
	}
	defer syscall.CloseHandle(th)

	var data syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(th, &data); err != nil {
		return false, nil
	}
	if data.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		return false, nil
	}
	if data.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 {
		var info FILE_ATTRIBUTE_TAG_INFO
		err := GetFileInformationByHandleEx(th, FileAttributeTagInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
		if err != nil || info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 || info.ReparseTag&nameSurrogateBit == 0 {
			return false, nil
		}
	}
	if !setAsideForDelete(th, objAttrs, &data) {
		return false, nil
	}

	if TestRenameatRetryFails {
		err = STATUS_ACCESS_DENIED
	} else {
		err = rename()
	}
	if err != nil {
		// Put the destination back. If something has taken its name since, it
		// stays where it was moved to, a stray rather than a loss.
		if p16, e := syscall.UTF16FromString(name); e == nil {
			renameTo(th, objAttrs.RootDirectory, p16[:len(p16)-1], false)
		}
		return true, err
	}

	// The name has changed hands. The moved copy goes with its last handle.
	SetFileInformationByHandle(
		th,
		FileDispositionInfo,
		unsafe.Pointer(&FILE_DISPOSITION_INFO{
			DeleteFile: true,
		}),
		uint32(unsafe.Sizeof(FILE_DISPOSITION_INFO{})),
	)
	return true, nil
}

func Linkat(olddirfd syscall.Handle, oldpath string, newdirfd syscall.Handle, newpath string) error {
	objAttrs := &OBJECT_ATTRIBUTES{}
	if err := objAttrs.init(olddirfd, oldpath); err != nil {
		return err
	}
	var h syscall.Handle
	err := NtOpenFile(
		&h,
		SYNCHRONIZE|FILE_WRITE_ATTRIBUTES,
		objAttrs,
		&IO_STATUS_BLOCK{},
		FILE_SHARE_DELETE|FILE_SHARE_READ|FILE_SHARE_WRITE,
		FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT|FILE_SYNCHRONOUS_IO_NONALERT,
	)
	if err != nil {
		return ntCreateFileError(err, 0)
	}
	defer syscall.CloseHandle(h)

	linkInfo := FILE_LINK_INFORMATION{
		RootDirectory: newdirfd,
	}
	p16, err := syscall.UTF16FromString(newpath)
	if err != nil {
		return err
	}
	if len(p16) > len(linkInfo.FileName) {
		return syscall.EINVAL
	}
	copy(linkInfo.FileName[:], p16)
	linkInfo.FileNameLength = uint32((len(p16) - 1) * 2)

	const (
		FileLinkInformation = 11
	)
	err = NtSetInformationFile(
		h,
		&IO_STATUS_BLOCK{},
		unsafe.Pointer(&linkInfo),
		uint32(unsafe.Sizeof(FILE_LINK_INFORMATION{})),
		FileLinkInformation,
	)
	if st, ok := err.(NTStatus); ok {
		return st.Errno()
	}
	return err
}

// SymlinkatFlags configure Symlinkat.
//
// Symbolic links have two properties: They may be directory or file links,
// and they may be absolute or relative.
//
// The Windows API defines flags describing these properties
// (SYMBOLIC_LINK_FLAG_DIRECTORY and SYMLINK_FLAG_RELATIVE),
// but the flags are passed to different system calls and
// do not have distinct values, so we define our own enumeration
// that permits expressing both.
type SymlinkatFlags uint

const (
	SYMLINKAT_DIRECTORY = SymlinkatFlags(1 << iota)
	SYMLINKAT_RELATIVE
)

func Symlinkat(oldname string, newdirfd syscall.Handle, newname string, flags SymlinkatFlags) error {
	// Temporarily acquire symlink-creating privileges if possible.
	// This is the behavior of CreateSymbolicLinkW.
	//
	// (When passed the SYMBOLIC_LINK_FLAG_ALLOW_UNPRIVILEGED_CREATE flag,
	// CreateSymbolicLinkW ignores errors in acquiring privileges, as we do here.)
	return withPrivilege("SeCreateSymbolicLinkPrivilege", func() error {
		return symlinkat(oldname, newdirfd, newname, flags)
	})
}

func symlinkat(oldname string, newdirfd syscall.Handle, newname string, flags SymlinkatFlags) error {
	oldnameu16, err := syscall.UTF16FromString(oldname)
	if err != nil {
		return err
	}
	oldnameu16 = oldnameu16[:len(oldnameu16)-1] // trim off terminal NUL

	var options uint32
	if flags&SYMLINKAT_DIRECTORY != 0 {
		options |= FILE_DIRECTORY_FILE
	} else {
		options |= FILE_NON_DIRECTORY_FILE
	}

	objAttrs := &OBJECT_ATTRIBUTES{}
	if err := objAttrs.init(newdirfd, newname); err != nil {
		return err
	}
	var h syscall.Handle
	err = NtCreateFile(
		&h,
		SYNCHRONIZE|FILE_WRITE_ATTRIBUTES|DELETE,
		objAttrs,
		&IO_STATUS_BLOCK{},
		nil,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
		FILE_CREATE,
		FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT|FILE_SYNCHRONOUS_IO_NONALERT|options,
		nil,
		0,
	)
	if err != nil {
		return ntCreateFileError(err, 0)
	}
	defer syscall.CloseHandle(h)

	// https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_reparse_data_buffer
	type reparseDataBufferT struct {
		_ structs.HostLayout

		ReparseTag        uint32
		ReparseDataLength uint16
		Reserved          uint16

		SubstituteNameOffset uint16
		SubstituteNameLength uint16
		PrintNameOffset      uint16
		PrintNameLength      uint16
		Flags                uint32
	}

	const (
		headerSize = uint16(unsafe.Offsetof(reparseDataBufferT{}.SubstituteNameOffset))
		bufferSize = uint16(unsafe.Sizeof(reparseDataBufferT{}))
	)

	// Data buffer containing a SymbolicLinkReparseBuffer followed by the link target.
	rdbbuf := make([]byte, bufferSize+uint16(2*len(oldnameu16)))

	rdb := (*reparseDataBufferT)(unsafe.Pointer(&rdbbuf[0]))
	rdb.ReparseTag = syscall.IO_REPARSE_TAG_SYMLINK
	rdb.ReparseDataLength = uint16(len(rdbbuf)) - uint16(headerSize)
	rdb.SubstituteNameOffset = 0
	rdb.SubstituteNameLength = uint16(2 * len(oldnameu16))
	rdb.PrintNameOffset = 0
	rdb.PrintNameLength = rdb.SubstituteNameLength
	if flags&SYMLINKAT_RELATIVE != 0 {
		rdb.Flags = SYMLINK_FLAG_RELATIVE
	}

	namebuf := rdbbuf[bufferSize:]
	copy(namebuf, unsafe.String((*byte)(unsafe.Pointer(&oldnameu16[0])), 2*len(oldnameu16)))

	var bytesReturned uint32
	err = syscall.DeviceIoControl(
		h,
		FSCTL_SET_REPARSE_POINT,
		&rdbbuf[0],
		uint32(len(rdbbuf)),
		nil,
		0,
		&bytesReturned,
		nil)
	if err != nil {
		// Creating the symlink has failed, so try to remove the file.
		const FileDispositionInformation = 13
		NtSetInformationFile(
			h,
			&IO_STATUS_BLOCK{},
			unsafe.Pointer(&FILE_DISPOSITION_INFORMATION{
				DeleteFile: true,
			}),
			uint32(unsafe.Sizeof(FILE_DISPOSITION_INFORMATION{})),
			FileDispositionInformation,
		)
		return err
	}

	return nil
}

// withPrivilege temporariliy acquires the named privilege and runs f.
// If the privilege cannot be acquired it runs f anyway,
// which should fail with an appropriate error.
func withPrivilege(privilege string, f func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := ImpersonateSelf(SecurityImpersonation)
	if err != nil {
		return f()
	}
	defer RevertToSelf()

	curThread, err := GetCurrentThread()
	if err != nil {
		return f()
	}
	var token syscall.Token
	err = OpenThreadToken(curThread, syscall.TOKEN_QUERY|TOKEN_ADJUST_PRIVILEGES, false, &token)
	if err != nil {
		return f()
	}
	defer syscall.CloseHandle(syscall.Handle(token))

	privStr, err := syscall.UTF16PtrFromString(privilege)
	if err != nil {
		return f()
	}
	var tokenPriv TOKEN_PRIVILEGES
	err = LookupPrivilegeValue(nil, privStr, &tokenPriv.Privileges[0].Luid)
	if err != nil {
		return f()
	}

	tokenPriv.PrivilegeCount = 1
	tokenPriv.Privileges[0].Attributes = SE_PRIVILEGE_ENABLED
	err = AdjustTokenPrivileges(token, false, &tokenPriv, 0, nil, nil)
	if err != nil {
		return f()
	}

	return f()
}
