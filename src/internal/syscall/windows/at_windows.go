// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package windows

import (
	"internal/oserror"
	"internal/strconv"
	"internal/stringslite"
	"runtime"
	"structs"
	"sync"
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
	if flag&O_NOFOLLOW_ANY != 0 {
		objAttrs.Attributes |= OBJ_DONT_REPARSE
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
	var err error
	if TestOpenatFallback && flag&O_NOFOLLOW_ANY != 0 {
		err = STATUS_INVALID_PARAMETER
	} else {
		err = NtCreateFile(
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
	if err == STATUS_INVALID_PARAMETER && flag&O_NOFOLLOW_ANY != 0 {
		h, err = openatFallback(name, SYNCHRONIZE|access, *objAttrs, fileAttrs, disposition, FILE_OPEN_FOR_BACKUP_INTENT|options)
	}
	if err != nil {
		return h, ntCreateFileError(err, flag)
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

// TestOpenatFallback should only be used for testing purposes.
// When set, Openat simulates a system that does not support OBJ_DONT_REPARSE.
var TestOpenatFallback bool

// openatFallback implements O_NOFOLLOW_ANY for a single path component on
// Windows versions that do not support OBJ_DONT_REPARSE (including Windows 10
// build 10240). See go.dev/issue/78131.
func openatFallback(name string, access uint32, attrs OBJECT_ATTRIBUTES, fileAttrs, disposition, options uint32) (syscall.Handle, error) {
	// FILE_OPEN_REPARSE_POINT only prevents following the final component.
	// All current production callers that request O_NOFOLLOW_ANY pass a
	// single component: os.Root splits paths (including symlink targets)
	// before opening each component, and RemoveAll passes a basename or a
	// directory entry name relative to an open parent directory. Thus this
	// fallback also supports multi-component paths passed to those APIs.
	// Reject other paths here rather than weaken O_NOFOLLOW_ANY: this is
	// not a general replacement for OBJ_DONT_REPARSE.
	if attrs.RootDirectory == 0 || name == ".." ||
		stringslite.IndexByte(name, '\\') >= 0 || stringslite.IndexByte(name, '/') >= 0 || stringslite.IndexByte(name, ':') >= 0 {
		return syscall.InvalidHandle, STATUS_INVALID_PARAMETER
	}
	attrs.Attributes &^= OBJ_DONT_REPARSE
	var h syscall.Handle
	err := NtCreateFile(
		&h, access, &attrs, &IO_STATUS_BLOCK{}, nil, fileAttrs,
		FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE, disposition,
		(options|FILE_OPEN_REPARSE_POINT)&^FILE_DELETE_ON_CLOSE, nil, 0,
	)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	// Inspect the handle, not the path, before truncation or delete-on-close.
	// Skip this check if opening the reparse point itself was requested,
	// or if O_CREAT|O_EXCL requires creating a new file without following links.
	if options&FILE_OPEN_REPARSE_POINT == 0 {
		info, err := fileAttributesAndTag(h)
		if err == nil && info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			err = STATUS_REPARSE_POINT_ENCOUNTERED
		}
		if err != nil {
			syscall.CloseHandle(h)
			return syscall.InvalidHandle, err
		}
	}
	if options&FILE_DELETE_ON_CLOSE == 0 {
		return h, nil
	}
	defer syscall.CloseHandle(h)

	// Only enable deletion after checking for a reparse point. An empty
	// name relative to h reopens the same file, even if it has been renamed
	// or its directory entry has been replaced since the first open.
	// The file already exists, including when created with O_EXCL.
	attrs.RootDirectory = h
	attrs.ObjectName = &NTUnicodeString{}
	var dh syscall.Handle
	err = NtOpenFile(
		&dh, access, &attrs, &IO_STATUS_BLOCK{},
		FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE,
		options|FILE_OPEN_REPARSE_POINT,
	)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	return dh, nil
}

// ntCreateFileError maps error returns from NTCreateFile to user-visible errors.
func ntCreateFileError(err error, flag uint64) error {
	s, ok := err.(NTStatus)
	if !ok {
		// The Openat fallback can also return Win32 errors.
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
		FILE_DIRECTORY_FILE|FILE_OPEN_REPARSE_POINT,
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
	return deleteat(objAttrs, name, options)
}

// fileAttributeTagInformation is the class for a handle's attributes and
// reparse tag alone.
const fileAttributeTagInformation = 35

// fileAttributesAndTag reads h's attributes and reparse tag in one call. A
// file system that refuses the class, a named pipe's among them, is asked for
// the attributes alone.
func fileAttributesAndTag(h syscall.Handle) (FILE_ATTRIBUTE_TAG_INFO, error) {
	var info FILE_ATTRIBUTE_TAG_INFO
	if NtQueryInformationFile(h, &IO_STATUS_BLOCK{}, unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)), fileAttributeTagInformation) == nil {
		return info, nil
	}
	var data syscall.ByHandleFileInformation
	err := syscall.GetFileInformationByHandle(h, &data)
	info.FileAttributes = data.FileAttributes
	return info, err
}

// A fileIdentity is a volume serial number and a file index.
type fileIdentity struct {
	volume uint32
	index  uint64
}

// aligned8 returns the first 8-byte aligned address in buf, which is made 8
// bytes longer for it. The kernel refuses an unaligned buffer for an answer
// with 64-bit fields, and 386 aligns locals to 4.
func aligned8(buf []byte) unsafe.Pointer {
	p := unsafe.Pointer(&buf[0])
	return unsafe.Add(p, (8-uintptr(p)%8)%8)
}

// identityOf reads the volume serial number and file index of the file h is
// open on.
func identityOf(h syscall.Handle) (fileIdentity, bool) {
	var d syscall.ByHandleFileInformation
	if syscall.GetFileInformationByHandle(h, &d) != nil {
		return fileIdentity{}, false
	}
	return fileIdentity{volume: d.VolumeSerialNumber, index: uint64(d.FileIndexHigh)<<32 | uint64(d.FileIndexLow)}, true
}

// ErrDeleteatHeld is Deletepath's answer for a file held open for I/O, which
// must be deleted through its parent so that it can be set aside.
var ErrDeleteatHeld error = deleteatHeld{}

type deleteatHeld struct{}

func (deleteatHeld) Error() string { return "file held open, delete from the parent directory" }

// DeleteFileFreesHeldName reports whether DeleteFileW frees the name of a file
// another handle holds at once, as a POSIX delete does. Some kernels take the
// POSIX class (KernelDeletesAsUpstream), yet their DeleteFileW does not.
var DeleteFileFreesHeldName = sync.OnceValue(func() bool {
	return kernelHas(probeDeleteFileFreesName)
})

// probeDeleteFileFreesName deletes a new file it holds open with DeleteFileW,
// as upstream's os.Remove does, and reports whether the name went at once.
func probeDeleteFileFreesName() bool {
	name, err := probeName("-remove")
	if err != nil {
		return false
	}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return false
	}
	const FILE_FLAG_DELETE_ON_CLOSE = 0x04000000
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ|DELETE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.CREATE_NEW, FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	if syscall.DeleteFile(p) != nil {
		return false
	}
	_, err = syscall.GetFileAttributes(p)
	return err == syscall.ERROR_FILE_NOT_FOUND
}

// Deletepath deletes the file at DOS path p as DeleteFileW resolves it, with
// no handle to its parent. It answers ErrDeleteatHeld for a held file, EINVAL
// for a name not on a disk and EISDIR for a directory.
func Deletepath(p *uint16) error {
	var nt NTUnicodeString
	if RtlDosPathNameToNtPathName_U(p, &nt, nil, nil) == 0 {
		return syscall.EINVAL
	}
	defer RtlFreeUnicodeString(&nt)
	if !onDiskName(&nt) {
		return syscall.EINVAL
	}
	objAttrs := &OBJECT_ATTRIBUTES{ObjectName: &nt}
	objAttrs.Length = uint32(unsafe.Sizeof(*objAttrs))
	// No name is needed, since this route never sets a file aside.
	return deleteat(objAttrs, "", FILE_NON_DIRECTORY_FILE)
}

// onDiskName reports whether nt names a drive, a volume or a share other than
// PIPE and MAILSLOT. Opening a pipe or a device would connect to it.
func onDiskName(nt *NTUnicodeString) bool {
	s := unsafe.Slice(nt.Buffer, nt.Length/2)
	fold := func(c uint16) uint16 {
		if 'a' <= c && c <= 'z' {
			c -= 'a' - 'A'
		}
		return c
	}
	prefix := func(s []uint16, p string) bool {
		if len(s) < len(p) {
			return false
		}
		for i := 0; i < len(p); i++ {
			if fold(s[i]) != uint16(p[i]) {
				return false
			}
		}
		return true
	}
	if !prefix(s, `\??\`) {
		return false
	}
	s = s[4:]
	switch {
	case len(s) >= 3 && 'A' <= fold(s[0]) && fold(s[0]) <= 'Z' && s[1] == ':' && s[2] == '\\':
		return true
	case prefix(s, "VOLUME{"):
		return true
	case prefix(s, `UNC\`):
		// \??\UNC\server\share\...: the share decides.
		s = s[4:]
		i := 0
		for i < len(s) && s[i] != '\\' {
			i++
		}
		if i == 0 || i == len(s) {
			return false
		}
		share := s[i+1:]
		j := 0
		for j < len(share) && share[j] != '\\' {
			j++
		}
		share = share[:j]
		return len(share) > 0 && !(len(share) == len("PIPE") && prefix(share, "PIPE")) &&
			!(len(share) == len("MAILSLOT") && prefix(share, "MAILSLOT"))
	}
	return false
}

// deleteat deletes the entry objAttrs names. name, relative to its parent,
// puts the file back if the delete is refused after moving it aside.
func deleteat(objAttrs *OBJECT_ATTRIBUTES, name string, options uint32) error {
	// Where the kernel takes the fallback (KernelDeletesAsUpstream), a
	// file's delete open shares deletion alone, so it fails exactly when
	// the file is held (see heldForIO). It also asks for write-attribute
	// access, to clear a read-only file on this handle.
	fallback := TestDeleteatFallback || !KernelDeletesAsUpstream()
	probe := fallback && options&FILE_NON_DIRECTORY_FILE != 0
	share := uint32(FILE_SHARE_DELETE | FILE_SHARE_READ | FILE_SHARE_WRITE)
	if probe {
		share = FILE_SHARE_DELETE
	}
	held := false
	writeAttrs := false
	var h syscall.Handle
	var err error
	var again, pending waiter
	for {
		open := func(access uint32) error {
			return NtOpenFile(
				&h,
				access,
				objAttrs,
				&IO_STATUS_BLOCK{},
				share,
				FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT|options,
			)
		}
		writeAttrs = probe
		if writeAttrs {
			err = open(FILE_READ_ATTRIBUTES | FILE_WRITE_ATTRIBUTES | DELETE)
			if err == STATUS_ACCESS_DENIED {
				writeAttrs = false
			}
		}
		if !writeAttrs {
			err = open(FILE_READ_ATTRIBUTES | DELETE)
		}
		if err == STATUS_ACCESS_DENIED {
			// Access denied, try opening with DELETE only.
			// This may succeed if the file has restrictive permissions
			// but the caller has delete child permission on the parent directory.
			err = open(DELETE)
		}
		if probe && err == STATUS_SHARING_VIOLATION {
			if objAttrs.RootDirectory == 0 {
				return ErrDeleteatHeld
			}
			probe, held = false, true
			share = FILE_SHARE_DELETE | FILE_SHARE_READ | FILE_SHARE_WRITE
			continue
		}
		// A sharing violation can be the file object of a just-run image
		// on its way out. See imageLingers.
		if err == STATUS_SHARING_VIOLATION && imageLingers() && again.again() {
			continue
		}
		// Without POSIX semantics a marked entry keeps its name until
		// its last handle closes, so wait a moment for it to go.
		if fallback && err == STATUS_DELETE_PENDING && pending.again() {
			continue
		}
		break
	}
	if err != nil {
		return ntCreateFileError(err, 0)
	}
	defer syscall.CloseHandle(h)

	if fallback {
		return deleteatLegacy(h, objAttrs, name, probe || held, held, writeAttrs)
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
		// The kernel takes the call, so the volume refused it (FAT32,
		// exFAT). Upstream's own fallback answers.
		return deleteatFallback(h)
	default:
		return err.(NTStatus).Errno()
	}
}

// TestDeleteatFallback should only be used for testing purposes.
// When set, [Deleteat] uses the fallback path unconditionally.
var TestDeleteatFallback bool

// KernelDeletesAsUpstream reports whether the kernel takes upstream's delete,
// the POSIX class with the flag that ignores the read-only attribute.
var KernelDeletesAsUpstream = sync.OnceValue(func() bool {
	return kernelHas(probeUpstreamDelete)
})

// kernelHas answers whether the kernel behaves as upstream counts on: no
// before Windows 10, yes from Windows 10 22H2 (build 19045), and on earlier
// Windows 10 whatever probe finds.
func kernelHas(probe func() bool) bool {
	major, _, build := KernelVersion()
	if major < 10 {
		return false
	}
	return build >= 19045 || probe()
}

// probeName returns a path in the temporary directory for a probe, which
// removes what it makes there. Unsure, a probe answers no.
func probeName(tag string) (string, error) {
	var buf [syscall.MAX_PATH]uint16
	n, err := syscall.GetTempPath(uint32(len(buf)), &buf[0])
	if err != nil {
		return "", err
	}
	if n >= uint32(len(buf)) {
		return "", syscall.ENAMETOOLONG
	}
	return syscall.UTF16ToString(buf[:n]) + "go-probe-" + strconv.Itoa(syscall.Getpid()) + tag, nil
}

// probeUpstreamDelete deletes a new read-only file as deleteat does where the
// kernel takes upstream's call, and reports whether it did.
func probeUpstreamDelete() bool {
	name, err := probeName("-delete")
	if err != nil {
		return false
	}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return false
	}
	h, err := syscall.CreateFile(p, DELETE|FILE_WRITE_ATTRIBUTES, 0, nil, syscall.CREATE_NEW, syscall.FILE_ATTRIBUTE_READONLY, 0)
	if err != nil {
		return false
	}
	const FileDispositionInformationEx = 64
	err = NtSetInformationFile(
		h,
		&IO_STATUS_BLOCK{},
		unsafe.Pointer(&FILE_DISPOSITION_INFORMATION_EX{
			Flags: FILE_DISPOSITION_DELETE |
				FILE_DISPOSITION_POSIX_SEMANTICS |
				FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE,
		}),
		uint32(unsafe.Sizeof(FILE_DISPOSITION_INFORMATION_EX{})),
		FileDispositionInformationEx,
	)
	if err != nil {
		// Refused, so make the file writable and delete it by name.
		info := FILE_BASIC_INFO{FileAttributes: syscall.FILE_ATTRIBUTE_NORMAL}
		SetFileInformationByHandle(h, FileBasicInfo, unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)))
	}
	syscall.CloseHandle(h)
	if err != nil {
		syscall.DeleteFile(p)
	}
	return err == nil
}

// TestDeleteatBeforeReopen should only be used for testing purposes. When set,
// the fallback calls it before it opens the name again to clear the read-only
// attribute.
var TestDeleteatBeforeReopen func()

// deleteatFallback is a deleteat implementation that strives
// for compatibility with older Windows versions and file systems
// over performance.
func deleteatFallback(h syscall.Handle) error {
	var data syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &data); err == nil && data.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		// Remove read-only attribute. Reopen the file, as it was previously open without FILE_WRITE_ATTRIBUTES access
		// in order to maximize compatibility in the happy path.
		wh, err := ReOpenFile(h,
			FILE_WRITE_ATTRIBUTES,
			FILE_SHARE_READ|FILE_SHARE_WRITE|FILE_SHARE_DELETE,
			syscall.FILE_FLAG_OPEN_REPARSE_POINT|syscall.FILE_FLAG_BACKUP_SEMANTICS,
		)
		if err != nil {
			return err
		}
		err = SetFileInformationByHandle(
			wh,
			FileBasicInfo,
			unsafe.Pointer(&FILE_BASIC_INFO{
				FileAttributes: data.FileAttributes &^ FILE_ATTRIBUTE_READONLY,
			}),
			uint32(unsafe.Sizeof(FILE_BASIC_INFO{})),
		)
		syscall.CloseHandle(wh)
		if err != nil {
			return err
		}
	}

	return SetFileInformationByHandle(
		h,
		FileDispositionInfo,
		unsafe.Pointer(&FILE_DISPOSITION_INFO{
			DeleteFile: true,
		}),
		uint32(unsafe.Sizeof(FILE_DISPOSITION_INFO{})),
	)
}

// deleteatLegacy is deleteat for a kernel without upstream's call (see
// KernelDeletesAsUpstream).
//
// known says the delete open has answered whether the file is held (held),
// and writeAttrs that h can clear the read-only attribute.
func deleteatLegacy(h syscall.Handle, objAttrs *OBJECT_ATTRIBUTES, name string, known, held, writeAttrs bool) error {
	var attrs uint32
	info, e := fileAttributesAndTag(h)
	haveData := e == nil
	if haveData {
		attrs = info.FileAttributes
		if attrs&syscall.FILE_ATTRIBUTE_READONLY != 0 {
			if err := clearReadOnly(h, writeAttrs, objAttrs, attrs); err != nil {
				return err
			}
		}
	}

	// Without POSIX semantics a name lives until its last handle closes,
	// so a held file is moved aside first. A directory is never moved. It
	// must fail while it has entries, and these kernels cannot rename a
	// held one.
	var id fileIdentity
	haveID := false
	aside := false
	if haveData && attrs&syscall.FILE_ATTRIBUTE_DIRECTORY == 0 && (held || !known && heldForIO(objAttrs)) {
		if id, haveID = identityOf(h); haveID {
			aside = setAsideForDelete(h, objAttrs, id)
		}
	}

	var err error
	var again, pending waiter
	for {
		err = SetFileInformationByHandle(
			h,
			FileDispositionInfo,
			unsafe.Pointer(&FILE_DISPOSITION_INFO{
				DeleteFile: true,
			}),
			uint32(unsafe.Sizeof(FILE_DISPOSITION_INFO{})),
		)
		// STATUS_CANNOT_DELETE (an image still mapped) arrives as
		// access denied. See imageLingers.
		if err == syscall.ERROR_ACCESS_DENIED && imageLingers() && again.again() {
			continue
		}
		// A directory whose entries are all marked for deletion
		// empties as their handles close, so wait a moment.
		if err == syscall.ERROR_DIR_NOT_EMPTY && !haveID {
			id, haveID = identityOf(h)
		}
		if err == syscall.ERROR_DIR_NOT_EMPTY && haveID && onlyDeletePending(objAttrs, id) && pending.again() {
			continue
		}
		break
	}
	if err != nil && aside {
		// The file stays. Put its name back unless something has taken
		// it since.
		if p16, err := syscall.UTF16FromString(name); err == nil {
			renameTo(h, objAttrs.RootDirectory, p16[:len(p16)-1], false)
		}
	}
	return err
}

// clearReadOnly clears the read-only attribute on h where it has
// write-attribute access, else through a fresh open of the name (ReOpenFile
// refuses a directory) if it still leads to the same file.
func clearReadOnly(h syscall.Handle, writeAttrs bool, objAttrs *OBJECT_ATTRIBUTES, attrs uint32) error {
	// Zero attributes means no change, so a file is given NORMAL instead.
	attrs &^= FILE_ATTRIBUTE_READONLY
	if attrs == 0 {
		attrs = syscall.FILE_ATTRIBUTE_NORMAL
	}
	if writeAttrs {
		return SetFileInformationByHandle(
			h,
			FileBasicInfo,
			unsafe.Pointer(&FILE_BASIC_INFO{
				FileAttributes: attrs,
			}),
			uint32(unsafe.Sizeof(FILE_BASIC_INFO{})),
		)
	}
	id, ok := identityOf(h)
	if !ok {
		// Without its identity the entry cannot be matched, so leave
		// the attribute and let the delete be refused.
		return nil
	}
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
	// Clear it only if the name still leads to the same file.
	if again, ok := identityOf(wh); ok && again == id {
		err = SetFileInformationByHandle(
			wh,
			FileBasicInfo,
			unsafe.Pointer(&FILE_BASIC_INFO{
				FileAttributes: attrs,
			}),
			uint32(unsafe.Sizeof(FILE_BASIC_INFO{})),
		)
	}
	syscall.CloseHandle(wh)
	return err
}

// imageLingers reports whether a just-run image stays mapped for a moment
// after its process exits, as on Windows 8 and 8.1.
var imageLingers = sync.OnceValue(func() bool {
	major, minor, _ := KernelVersion()
	return major == 6 && minor >= 2
})

// A waiter bounds one delete's retries at 100 ms, sleeping 1 ms between them.
// It reads the clock only once a retry is needed.
type waiter struct {
	deadline int64 // nanoseconds, zero until the first retry
}

// again sleeps a millisecond and reports whether Deleteat should try again.
func (w *waiter) again() bool {
	var now syscall.Filetime
	syscall.GetSystemTimeAsFileTime(&now)
	if w.deadline == 0 {
		w.deadline = now.Nanoseconds() + 100e6
	} else if now.Nanoseconds() >= w.deadline {
		return false
	}
	Sleep(1)
	return true
}

// onlyDeletePending reports whether every entry the directory still lists is
// pending deletion (STATUS_DELETE_PENDING, or STATUS_OBJECT_NAME_NOT_FOUND,
// which some kernels answer instead). It reopens the directory by name,
// which must still lead to the same directory.
func onlyDeletePending(objAttrs *OBJECT_ATTRIBUTES, id fileIdentity) bool {
	var dir syscall.Handle
	err := NtOpenFile(
		&dir,
		SYNCHRONIZE|FILE_LIST_DIRECTORY|FILE_READ_ATTRIBUTES,
		objAttrs,
		&IO_STATUS_BLOCK{},
		FILE_SHARE_DELETE|FILE_SHARE_READ|FILE_SHARE_WRITE,
		FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT|FILE_DIRECTORY_FILE|FILE_SYNCHRONOUS_IO_NONALERT,
	)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(dir)
	if again, ok := identityOf(dir); !ok || again != id {
		return false
	}
	// See aligned8.
	var bufRaw [4096 + 8]byte
	buf := aligned8(bufRaw[:])
	for {
		err := GetFileInformationByHandleEx(dir, FileIdBothDirectoryInfo, (*byte)(buf), 4096)
		if err == syscall.ERROR_NO_MORE_FILES {
			// Nothing listed, yet not empty. The entry went
			// between the calls, or the kernel hides marked
			// entries while counting them, as some kernels do.
			return true
		}
		if err != nil {
			return false
		}
		for off := uint32(0); ; {
			info := (*FILE_ID_BOTH_DIR_INFO)(unsafe.Add(buf, off))
			name := unsafe.Slice(&info.FileName[0], info.FileNameLength/2)
			if !(len(name) == 1 && name[0] == '.') && !(len(name) == 2 && name[0] == '.' && name[1] == '.') {
				entryAttrs := &OBJECT_ATTRIBUTES{}
				if err := entryAttrs.init(dir, syscall.UTF16ToString(name)); err != nil {
					return false
				}
				var eh syscall.Handle
				err := NtOpenFile(
					&eh,
					FILE_READ_ATTRIBUTES,
					entryAttrs,
					&IO_STATUS_BLOCK{},
					FILE_SHARE_DELETE|FILE_SHARE_READ|FILE_SHARE_WRITE,
					FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT,
				)
				if err == nil {
					syscall.CloseHandle(eh)
				}
				if err != STATUS_DELETE_PENDING && err != STATUS_OBJECT_NAME_NOT_FOUND {
					return false
				}
			}
			if info.NextEntryOffset == 0 {
				break
			}
			off += info.NextEntryOffset
		}
	}
}

// deleteStagingDir is where a file that cannot go at once is parked, out of
// the tree being removed. It is opened per use, since a handle kept open would
// block renaming or removing the program's temporary directory.
type deleteStagingDir struct {
	path   []uint16 // NUL-terminated
	volume uint32
	ok     bool
}

// open returns a handle for a rename into the directory, or 0.
func (s deleteStagingDir) open() syscall.Handle {
	h, err := syscall.CreateFile(
		&s.path[0],
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return 0
	}
	return h
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
	id, ok := identityOf(h)
	syscall.CloseHandle(h)
	if !ok {
		return
	}
	return deleteStagingDir{path: buf[:n+1], volume: id.volume, ok: true}
})

// heldForIO reports whether another handle has the file open for I/O. An open
// asking delete access alone and sharing deletion alone fails exactly then.
// Any other failure counts as held, which costs only a rename.
func heldForIO(objAttrs *OBJECT_ATTRIBUTES) bool {
	var h syscall.Handle
	err := NtOpenFile(
		&h,
		DELETE,
		objAttrs,
		&IO_STATUS_BLOCK{},
		FILE_SHARE_DELETE,
		FILE_OPEN_REPARSE_POINT|FILE_OPEN_FOR_BACKUP_INTENT,
	)
	if err != nil {
		return true
	}
	syscall.CloseHandle(h)
	return false
}

// setAsideForDelete moves the file open on h off its name, into the staging
// directory where it can. It reports whether that happened, since a rename
// cannot cross volumes and needs every handle to share deletion.
func setAsideForDelete(h syscall.Handle, objAttrs *OBJECT_ATTRIBUTES, id fileIdentity) bool {
	dir := objAttrs.RootDirectory
	if s := deleteStaging(); s.ok && s.volume == id.volume {
		if sh := s.open(); sh != 0 {
			defer syscall.CloseHandle(sh)
			dir = sh
		}
	}
	if dir == 0 {
		return false
	}
	// The file index is unique on the volume, so two names cannot collide.
	const hex = "0123456789abcdef"
	name := make([]uint16, 0, len(".go-deleted-")+16)
	for _, c := range ".go-deleted-" {
		name = append(name, uint16(c))
	}
	for _, v := range [2]uint32{uint32(id.index >> 32), uint32(id.index)} {
		for shift := 28; shift >= 0; shift -= 4 {
			name = append(name, uint16(hex[(v>>uint(shift))&0xf]))
		}
	}
	return renameTo(h, dir, name, true) == nil
}

// renameTo renames the file open on h to name in dir, on the same volume,
// replacing what is there only if replace is set.
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
	// Where a POSIX rename cannot replace a link (posixRenameReplacesLinks)
	// a target is replaced by hand, which needs FILE_READ_ATTRIBUTES. If that is
	// refused, ask for only what the rename needs.
	byHand := TestRenameatNoPosixSemantics || !posixRenameReplacesLinks()
	access := uint32(SYNCHRONIZE | DELETE)
	if byHand {
		access |= FILE_READ_ATTRIBUTES
	}
	err := open(access)
	if err == STATUS_ACCESS_DENIED && byHand {
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
	if !byHand {
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
	if err == STATUS_ACCESS_DENIED && byHand && !isDirectory(h) {
		// The plain rename cannot replace a link (it counts as a
		// directory) or a held file. A directory source is left to
		// it.
		if replaced, rerr := replaceRenameTarget(newdirfd, newpath, renameFile); replaced {
			err = rerr
		}
	}
	if st, ok := err.(NTStatus); ok {
		return st.Errno()
	}
	return err
}

// isDirectory reports whether h is a directory itself, not a link standing in
// for one. Unsure, it says yes, which only leaves a target in place.
func isDirectory(h syscall.Handle) bool {
	info, err := fileAttributesAndTag(h)
	if err != nil {
		return true
	}
	const dirOrLink = syscall.FILE_ATTRIBUTE_DIRECTORY | syscall.FILE_ATTRIBUTE_REPARSE_POINT
	return info.FileAttributes&dirOrLink == syscall.FILE_ATTRIBUTE_DIRECTORY
}

// TestRenameatNoPosixSemantics should only be used for testing purposes.
// When set, [Renameat] replaces a target by hand, as where a POSIX rename
// cannot replace a link.
var TestRenameatNoPosixSemantics bool

// TestRenameatRetryFails should only be used for testing purposes. When set,
// the rename retried after the target was moved aside fails.
var TestRenameatRetryFails bool

// posixRenameReplacesLinks reports whether a POSIX rename replaces a link.
// Some early versions of Windows 10 have POSIX rename yet refuse that.
var posixRenameReplacesLinks = sync.OnceValue(func() bool {
	return kernelHas(probePosixRenameOntoLink)
})

// probePosixRenameOntoLink renames a new file onto a new junction with POSIX
// semantics, as Renameat does, and reports whether the junction was replaced.
func probePosixRenameOntoLink() bool {
	base, err := probeName("-rename")
	if err != nil {
		return false
	}
	dir, err1 := syscall.UTF16PtrFromString(base + "-dir")
	link, err2 := syscall.UTF16PtrFromString(base + "-link")
	file, err3 := syscall.UTF16PtrFromString(base + "-file")
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	if syscall.CreateDirectory(dir, nil) != nil {
		return false
	}
	defer syscall.RemoveDirectory(dir)
	if !makeJunction(link, base+"-dir") {
		return false
	}
	h, err := syscall.CreateFile(file, DELETE|SYNCHRONIZE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.CREATE_NEW, 0, 0)
	if err != nil {
		syscall.RemoveDirectory(link)
		return false
	}
	info := FILE_RENAME_INFORMATION_EX{
		Flags: FILE_RENAME_REPLACE_IF_EXISTS | FILE_RENAME_POSIX_SEMANTICS,
	}
	nt, err := syscall.UTF16FromString(`\??\` + base + "-link")
	if err == nil && len(nt) <= len(info.FileName) {
		copy(info.FileName[:], nt)
		info.FileNameLength = uint32((len(nt) - 1) * 2)
		const FileRenameInformationEx = 65
		err = NtSetInformationFile(h, &IO_STATUS_BLOCK{}, unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)), FileRenameInformationEx)
	} else if err == nil {
		err = syscall.ENAMETOOLONG
	}
	syscall.CloseHandle(h)
	if err == nil {
		syscall.DeleteFile(link)
		return true
	}
	syscall.DeleteFile(file)
	syscall.RemoveDirectory(link)
	return false
}

// makeJunction makes the directory link, a junction to the absolute path
// target, and reports whether it did. It leaves nothing behind when it fails.
func makeJunction(link *uint16, target string) bool {
	sub, err := syscall.UTF16FromString(`\??\` + target)
	if err != nil || syscall.CreateDirectory(link, nil) != nil {
		return false
	}
	h, err := syscall.CreateFile(link, syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OPEN_REPARSE_POINT|syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err == nil {
		// The reparse header, the name offsets, the name with its NUL and
		// an empty print name.
		n := 2 * len(sub)
		buf := make([]byte, 16+n+2)
		*(*uint32)(unsafe.Pointer(&buf[0])) = IO_REPARSE_TAG_MOUNT_POINT
		*(*uint16)(unsafe.Pointer(&buf[4])) = uint16(len(buf) - 8)
		*(*uint16)(unsafe.Pointer(&buf[10])) = uint16(n - 2)
		*(*uint16)(unsafe.Pointer(&buf[12])) = uint16(n)
		copy(buf[16:], unsafe.Slice((*byte)(unsafe.Pointer(&sub[0])), n))
		var returned uint32
		err = syscall.DeviceIoControl(h, FSCTL_SET_REPARSE_POINT, &buf[0], uint32(len(buf)), nil, 0, &returned, nil)
		syscall.CloseHandle(h)
	}
	if err != nil {
		syscall.RemoveDirectory(link)
		return false
	}
	return true
}

// nameSurrogateBit marks the tag of a symlink or mount point, as opposed to a
// reparse point carrying data for a filter driver.
const nameSurrogateBit = 0x20000000

// replaceRenameTarget moves the destination aside, renames again, and
// deletes the moved copy, or moves it back on failure. It leaves what POSIX
// rename refuses: a directory that is not a link, and a read-only file. It
// reports whether the second rename was reached, and its result.
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

	info, err := fileAttributesAndTag(th)
	if err != nil {
		return false, nil
	}
	if info.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		return false, nil
	}
	if info.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0 &&
		(info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 || info.ReparseTag&nameSurrogateBit == 0) {
		return false, nil
	}
	id, ok := identityOf(th)
	if !ok || !setAsideForDelete(th, objAttrs, id) {
		return false, nil
	}

	if TestRenameatRetryFails {
		err = STATUS_ACCESS_DENIED
	} else {
		err = rename()
	}
	if err != nil {
		// Put the target back unless its name was taken since.
		if p16, e := syscall.UTF16FromString(name); e == nil {
			renameTo(th, objAttrs.RootDirectory, p16[:len(p16)-1], false)
		}
		return true, err
	}

	// The rename succeeded. The moved copy is deleted when its last handle
	// closes.
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
