// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package windows

import "syscall"

var RenameTo = renameTo

func OnDiskName(name string) bool {
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return false
	}
	var nt NTUnicodeString
	if RtlDosPathNameToNtPathName_U(p, &nt, nil, nil) == 0 {
		return false
	}
	defer RtlFreeUnicodeString(&nt)
	return onDiskName(&nt)
}
