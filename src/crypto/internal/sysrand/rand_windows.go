// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sysrand

import (
	"internal/syscall/windows"
	"sync"
)

// ProcessPrng arrived in Windows 8. Earlier versions have RtlGenRandom.
var useProcessPrng = sync.OnceValue(func() bool {
	return windows.ErrorLoadingProcessPrng() == nil
})

func read(b []byte) error {
	if useProcessPrng() {
		return windows.ProcessPrng(b)
	}
	// RtlGenRandom only returns 1<<32-1 bytes at a time. We only read at
	// most 1<<31-1 bytes at a time so that  this works the same on 32-bit
	// and 64-bit systems.
	return batched(windows.RtlGenRandom, 1<<31-1)(b)
}

// batched returns a function that calls f to populate a []byte by chunking it
// into subslices of, at most, readMax bytes.
func batched(f func([]byte) error, readMax int) func([]byte) error {
	return func(out []byte) error {
		for len(out) > 0 {
			read := len(out)
			if read > readMax {
				read = readMax
			}
			if err := f(out[:read]); err != nil {
				return err
			}
			out = out[read:]
		}
		return nil
	}
}
