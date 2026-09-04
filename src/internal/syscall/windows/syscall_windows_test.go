// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package windows_test

import (
	"bytes"
	"internal/syscall/windows"
	"strings"
	"testing"
	"unicode/utf16"
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

func TestUTF16PtrToStringAllocs(t *testing.T) {
	msg := "Hello, world 🐻"
	testUTF16PtrToStringAllocs(t, msg)
	testUTF16PtrToStringAllocs(t, strings.Repeat(msg, 10))
}

func testUTF16PtrToStringAllocs(t *testing.T, msg string) {
	in := utf16.Encode([]rune(msg + "\x00"))
	var out string
	alloccnt := testing.AllocsPerRun(1000, func() {
		out = windows.UTF16PtrToString(&in[0])
	})
	if out != msg {
		t.Errorf("windows.UTF16PtrToString(%v) returned %q; want %q", in, out, msg)
	}
	if alloccnt > 1.01 {
		t.Errorf("windows.UTF16PtrToString(%v) made %v allocs per call; want 1", in, alloccnt)
	}
}
