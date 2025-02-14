// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sysrand

import "internal/syscall/windows"

func read(b []byte) error {
	const maxChunk = 1<<31 - 1
	for len(b) > 0 {
		chunk := b
		if len(chunk) > maxChunk {
			chunk = chunk[:maxChunk]
		}
		if err := windows.RtlGenRandom(chunk); err != nil {
			return err
		}
		b = b[len(chunk):]
	}
	return nil
}
