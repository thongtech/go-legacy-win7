// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sysrand

import (
	"bytes"
	"testing"
)

func TestBatched(t *testing.T) {
	fillBatched := batched(func(p []byte) error {
		for i := range p {
			p[i] = byte(i)
		}
		return nil
	}, 5)

	p := make([]byte, 13)
	if err := fillBatched(p); err != nil {
		t.Fatalf("batched function returned error: %s", err)
	}
	expected := []byte{0, 1, 2, 3, 4, 0, 1, 2, 3, 4, 0, 1, 2}
	if !bytes.Equal(expected, p) {
		t.Errorf("incorrect batch result: got %x, want %x", p, expected)
	}
}
