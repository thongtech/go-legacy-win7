// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sysrand

import (
	"internal/syscall/windows"
	"sync"
)

// ProcessPrng is only available since Windows 8. Fall back to RtlGenRandom,
// which every earlier version exports from advapi32.dll.
var useProcessPrng = sync.OnceValue(func() bool {
	return windows.ErrorLoadingProcessPrng() == nil
})

func read(b []byte) error {
	if useProcessPrng() {
		return windows.ProcessPrng(b)
	}
	return readBatched(b, 1<<31-1)
}

// readBatched fills b through RtlGenRandom, whose length is a ULONG. A
// longer buffer would be truncated to its low 32 bits and the rest left
// as it was, reported as a success. Read max bytes at a time. ProcessPrng
// takes a SIZE_T and needs none of this.
func readBatched(b []byte, max int) error {
	for len(b) > 0 {
		chunk := b
		if len(chunk) > max {
			chunk = chunk[:max]
		}
		if err := windows.RtlGenRandom(chunk); err != nil {
			return err
		}
		b = b[len(chunk):]
	}
	return nil
}
