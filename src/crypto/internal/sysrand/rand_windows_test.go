// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sysrand

import "testing"

// TestReadBatched checks that a buffer longer than one RtlGenRandom call
// may take is filled to its end. The real limit is 1<<31-1, too large to
// allocate here, so a small one stands in for it.
func TestReadBatched(t *testing.T) {
	b := make([]byte, 1000)
	if err := readBatched(b, 7); err != nil {
		t.Fatal(err)
	}
	// The tail lies well past the first chunk, so it is only written if
	// the loop advanced. All zeroes there means it did not.
	tail := b[len(b)-32:]
	for _, c := range tail {
		if c != 0 {
			return
		}
	}
	t.Error("the end of the buffer was left unwritten")
}
