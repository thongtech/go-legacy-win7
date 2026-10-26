// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build race

// The race runtime calls WaitOnAddress, WakeByAddressSingle and
// WakeByAddressAll (Windows 8) by name, not through __imp_, so defining them
// here resolves them at link time with no import of the API set. Forward to
// the OS where it has them, emulate where it does not.

typedef int BOOL;
typedef unsigned long DWORD;
typedef unsigned long long SIZE_T;
typedef unsigned long long UINTPTR;
typedef void *PVOID;
typedef void *HMODULE;

typedef struct {
	PVOID Ptr;
} SRWLOCK;

typedef struct {
	PVOID Ptr;
} CONDITION_VARIABLE;

// Declared here, since windows.h marks the three functions this file defines
// dllimport, and a dllimport function cannot be defined.
extern void __stdcall AcquireSRWLockExclusive(SRWLOCK *lock);
extern void __stdcall ReleaseSRWLockExclusive(SRWLOCK *lock);
extern BOOL __stdcall SleepConditionVariableSRW(CONDITION_VARIABLE *cond, SRWLOCK *lock, DWORD milliseconds, DWORD flags);
extern void __stdcall WakeAllConditionVariable(CONDITION_VARIABLE *cond);
extern HMODULE __stdcall GetModuleHandleA(const char *name);
extern PVOID __stdcall GetProcAddress(HMODULE module, const char *name);

BOOL __stdcall WaitOnAddress(volatile PVOID address, PVOID compareAddress, SIZE_T addressSize, DWORD milliseconds);
void __stdcall WakeByAddressSingle(PVOID address);
void __stdcall WakeByAddressAll(PVOID address);

typedef BOOL(__stdcall *waitOnAddressFunc)(volatile PVOID, PVOID, SIZE_T, DWORD);
typedef void(__stdcall *wakeByAddressFunc)(PVOID);

static waitOnAddressFunc waitOnAddressNative;
static wakeByAddressFunc wakeByAddressSingleNative;
static wakeByAddressFunc wakeByAddressAllNative;
static int probed;

// probeOnce looks the three functions up in KernelBase, where the API set
// forwards them. Racing probes write the same values.
static void
probeOnce(void)
{
	HMODULE kernelbase;

	if (__atomic_load_n(&probed, __ATOMIC_ACQUIRE))
		return;
	kernelbase = GetModuleHandleA("kernelbase.dll");
	if (kernelbase != 0) {
		waitOnAddressFunc wait = (waitOnAddressFunc)GetProcAddress(kernelbase, "WaitOnAddress");
		wakeByAddressFunc single = (wakeByAddressFunc)GetProcAddress(kernelbase, "WakeByAddressSingle");
		wakeByAddressFunc all = (wakeByAddressFunc)GetProcAddress(kernelbase, "WakeByAddressAll");

		// Take all three or none, since waiting natively and waking by
		// hand, or the other way around, would lose wakeups.
		if (wait != 0 && single != 0 && all != 0) {
			waitOnAddressNative = wait;
			wakeByAddressSingleNative = single;
			wakeByAddressAllNative = all;
		}
	}
	__atomic_store_n(&probed, 1, __ATOMIC_RELEASE);
}

#define NBUCKET 64

static struct {
	SRWLOCK lock;
	CONDITION_VARIABLE cond;
} buckets[NBUCKET];

// Addresses sharing a bucket wake each other spuriously, which the contract
// allows.
static int
bucketFor(PVOID address)
{
	UINTPTR v = (UINTPTR)address;

	return (int)(((v >> 3) ^ (v >> 13)) % NBUCKET);
}

static BOOL
unchanged(volatile PVOID address, PVOID compareAddress, SIZE_T addressSize)
{
	switch (addressSize) {
	case 1:
		return *(volatile unsigned char *)address == *(unsigned char *)compareAddress;
	case 2:
		return *(volatile unsigned short *)address == *(unsigned short *)compareAddress;
	case 4:
		return *(volatile unsigned int *)address == *(unsigned int *)compareAddress;
	case 8:
		return *(volatile unsigned long long *)address == *(unsigned long long *)compareAddress;
	}
	return 0;
}

BOOL __stdcall
WaitOnAddress(volatile PVOID address, PVOID compareAddress, SIZE_T addressSize, DWORD milliseconds)
{
	int i;
	BOOL woken;

	probeOnce();
	if (waitOnAddressNative != 0)
		return waitOnAddressNative(address, compareAddress, addressSize, milliseconds);

	i = bucketFor((PVOID)address);
	AcquireSRWLockExclusive(&buckets[i].lock);
	woken = 1;
	if (unchanged(address, compareAddress, addressSize)) {
		// SleepConditionVariableSRW sets ERROR_TIMEOUT itself, as a
		// caller of WaitOnAddress expects.
		woken = SleepConditionVariableSRW(&buckets[i].cond, &buckets[i].lock, milliseconds, 0);
	}
	ReleaseSRWLockExclusive(&buckets[i].lock);
	return woken;
}

// wakeBucket wakes every waiter on address's bucket. A waiter that has read
// the address but not yet slept holds the lock, so it cannot miss the wake.
static void
wakeBucket(PVOID address)
{
	int i = bucketFor(address);

	AcquireSRWLockExclusive(&buckets[i].lock);
	ReleaseSRWLockExclusive(&buckets[i].lock);
	WakeAllConditionVariable(&buckets[i].cond);
}

void __stdcall
WakeByAddressSingle(PVOID address)
{
	probeOnce();
	if (wakeByAddressSingleNative != 0) {
		wakeByAddressSingleNative(address);
		return;
	}
	// Waking all is allowed, since callers recheck the address anyway.
	wakeBucket(address);
}

void __stdcall
WakeByAddressAll(PVOID address)
{
	probeOnce();
	if (wakeByAddressAllNative != 0) {
		wakeByAddressAllNative(address);
		return;
	}
	wakeBucket(address);
}
