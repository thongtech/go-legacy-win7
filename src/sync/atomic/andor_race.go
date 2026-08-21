// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build race

// And/Or under the race detector are implemented with Load+CAS loops so that
// the legacy Windows race_windows.syso (which lacks
// __tsan_go_atomic*_fetch_and/or) can still link. Race instrumentation comes
// from the existing Load/CompareAndSwap TSAN hooks.

package atomic

func AndInt32(addr *int32, mask int32) (old int32) {
	for {
		old = LoadInt32(addr)
		if CompareAndSwapInt32(addr, old, old&mask) {
			return old
		}
	}
}

func AndUint32(addr *uint32, mask uint32) (old uint32) {
	for {
		old = LoadUint32(addr)
		if CompareAndSwapUint32(addr, old, old&mask) {
			return old
		}
	}
}

func AndInt64(addr *int64, mask int64) (old int64) {
	for {
		old = LoadInt64(addr)
		if CompareAndSwapInt64(addr, old, old&mask) {
			return old
		}
	}
}

func AndUint64(addr *uint64, mask uint64) (old uint64) {
	for {
		old = LoadUint64(addr)
		if CompareAndSwapUint64(addr, old, old&mask) {
			return old
		}
	}
}

func AndUintptr(addr *uintptr, mask uintptr) (old uintptr) {
	for {
		old = LoadUintptr(addr)
		if CompareAndSwapUintptr(addr, old, old&mask) {
			return old
		}
	}
}

func OrInt32(addr *int32, mask int32) (old int32) {
	for {
		old = LoadInt32(addr)
		if CompareAndSwapInt32(addr, old, old|mask) {
			return old
		}
	}
}

func OrUint32(addr *uint32, mask uint32) (old uint32) {
	for {
		old = LoadUint32(addr)
		if CompareAndSwapUint32(addr, old, old|mask) {
			return old
		}
	}
}

func OrInt64(addr *int64, mask int64) (old int64) {
	for {
		old = LoadInt64(addr)
		if CompareAndSwapInt64(addr, old, old|mask) {
			return old
		}
	}
}

func OrUint64(addr *uint64, mask uint64) (old uint64) {
	for {
		old = LoadUint64(addr)
		if CompareAndSwapUint64(addr, old, old|mask) {
			return old
		}
	}
}

func OrUintptr(addr *uintptr, mask uintptr) (old uintptr) {
	for {
		old = LoadUintptr(addr)
		if CompareAndSwapUintptr(addr, old, old|mask) {
			return old
		}
	}
}
