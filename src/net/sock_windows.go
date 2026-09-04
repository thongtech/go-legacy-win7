// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"internal/syscall/windows"
	"os"
	"syscall"
)

func maxListenerBacklog() int {
	// When the socket backlog is SOMAXCONN, Windows will set the backlog to
	// "a reasonable maximum value".
	// See: https://learn.microsoft.com/en-us/windows/win32/api/winsock2/nf-winsock2-listen
	return syscall.SOMAXCONN
}

func sysSocket(family, sotype, proto int) (syscall.Handle, error) {
	s, err := wsaSocketFunc(int32(family), int32(sotype), int32(proto),
		nil, 0, windows.WSA_FLAG_OVERLAPPED|windows.WSA_FLAG_NO_HANDLE_INHERIT)
	if err == nil {
		return s, nil
	}
	// WSA_FLAG_NO_HANDLE_INHERIT needs Windows 7 SP1, and WSASocket rejects the
	// flag outright without it. Create an inheritable socket instead and clear
	// the flag afterwards. Unlike the equivalent code before Go 1.22 this does
	// not take syscall.ForkLock, since a child only inherits the handles listed
	// in PROC_THREAD_ATTRIBUTE_HANDLE_LIST.
	s, err = wsaSocketFunc(int32(family), int32(sotype), int32(proto),
		nil, 0, windows.WSA_FLAG_OVERLAPPED)
	if err != nil {
		return syscall.InvalidHandle, os.NewSyscallError("socket", err)
	}
	syscall.CloseOnExec(s)
	return s, nil
}
