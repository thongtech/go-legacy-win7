// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Export guts for testing.

package runtime

import (
	"internal/runtime/syscall/windows"
	"unsafe"
)

var (
	OsYield                 = osyield
	TimeBeginPeriodRetValue = &timeBeginPeriodRetValue
)

func NumberOfProcessors() int32 {
	var info windows.SystemInfo
	stdcall(_GetSystemInfo, uintptr(unsafe.Pointer(&info)))
	return int32(info.NumberOfProcessors)
}

func GetCallerFp() uintptr {
	return getcallerfp()
}

func LoadSystemLib(name []uint16) uintptr {
	return windowsLoadSystemLib(name)
}

func LoadSystemLibFromSysDir(name []uint16) uintptr {
	return loadSystemLibFromSysDir(name)
}

// ReadRandomFromRtlGenRandom reads through RtlGenRandom, loaded the way
// loadOptionalSyscalls loads it when ProcessPrng is unavailable.
func ReadRandomFromRtlGenRandom(r []byte) int {
	fn := loadRtlGenRandom()
	if fn == nil {
		return 0
	}
	return readRandomFrom(fn, r)
}
