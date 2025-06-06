// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build openbsd && !mips64

package unix

import (
	"internal/abi"
	"syscall"
	"unsafe"
)

//go:cgo_import_dynamic libc_readlinkat readlinkat "libc.so"

func libc_readlinkat_trampoline()

func Readlinkat(dirfd int, path string, buf []byte) (int, error) {
	p0, err := syscall.BytePtrFromString(path)
	if err != nil {
		return 0, err
	}
	var p1 unsafe.Pointer
	if len(buf) > 0 {
		p1 = unsafe.Pointer(&buf[0])
	} else {
		p1 = unsafe.Pointer(&_zero)
	}
	n, _, errno := syscall_syscall6(abi.FuncPCABI0(libc_readlinkat_trampoline), uintptr(dirfd), uintptr(unsafe.Pointer(p0)), uintptr(p1), uintptr(len(buf)), 0, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(n), nil
}

//go:cgo_import_dynamic libc_mkdirat mkdirat "libc.so"

func libc_mkdirat_trampoline()

func Mkdirat(dirfd int, path string, mode uint32) error {
	p, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}
	_, _, errno := syscall_syscall6(abi.FuncPCABI0(libc_mkdirat_trampoline), uintptr(dirfd), uintptr(unsafe.Pointer(p)), uintptr(mode), 0, 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
