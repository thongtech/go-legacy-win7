// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"internal/syscall/windows"
	"os"
	"sync"
	"syscall"
)

// kernelLacksNoHandleInherit reports whether ws2_32 lacks the flag, as Windows
// 7 before Service Pack 1 does. Elsewhere a refusal comes from a provider and
// stands.
var kernelLacksNoHandleInherit = sync.OnceValue(func() bool {
	major, minor, build := windows.KernelVersion()
	return major == 6 && minor == 1 && build < 7601
})

func maxListenerBacklog() int {
	// When the socket backlog is SOMAXCONN, Windows will set the backlog to
	// "a reasonable maximum value".
	// See: https://learn.microsoft.com/en-us/windows/win32/api/winsock2/nf-winsock2-listen
	return syscall.SOMAXCONN
}

// wsaSocket makes a socket as sysSocket and dupSocket ask. Where ws2_32 lacks
// WSA_FLAG_NO_HANDLE_INHERIT the socket is made inheritable and the flag
// cleared, with no ForkLock, since a child inherits only the handles in its
// handle list.
func wsaSocket(family, sotype, proto int32, info *syscall.WSAProtocolInfo) (syscall.Handle, error) {
	if !kernelLacksNoHandleInherit() {
		return wsaSocketFunc(family, sotype, proto, info, 0, windows.WSA_FLAG_OVERLAPPED|windows.WSA_FLAG_NO_HANDLE_INHERIT)
	}
	s, err := wsaSocketFunc(family, sotype, proto, info, 0, windows.WSA_FLAG_OVERLAPPED)
	if err == nil {
		syscall.CloseOnExec(s)
	}
	return s, err
}

func sysSocket(family, sotype, proto int) (syscall.Handle, error) {
	s, err := wsaSocket(int32(family), int32(sotype), int32(proto), nil)
	if err != nil {
		return syscall.InvalidHandle, os.NewSyscallError("socket", err)
	}
	return s, nil
}
