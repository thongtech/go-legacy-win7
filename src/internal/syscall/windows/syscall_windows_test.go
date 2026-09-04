// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package windows_test

import (
	"bytes"
	"internal/syscall/windows"
	"testing"
)

// TestRtlGenRandom checks the generator crypto/internal/sysrand falls back
// to when ProcessPrng is unavailable. It is called directly, so the fallback
// is covered on a host that has ProcessPrng as well.
func TestRtlGenRandom(t *testing.T) {
	b := make([]byte, 32)
	if err := windows.RtlGenRandom(b); err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(b, make([]byte, len(b))) {
		t.Error("RtlGenRandom left the buffer zeroed")
	}
}
