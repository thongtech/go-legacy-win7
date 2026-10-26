// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package syscall_test

import (
	"fmt"
	"internal/syscall/windows"
	"internal/testenv"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestOpen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "a")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	tests := []struct {
		path string
		flag int
		err  error
	}{
		{dir, syscall.O_RDONLY, nil},
		{dir, syscall.O_CREAT, nil},
		{dir, syscall.O_RDONLY | syscall.O_CREAT, nil},
		{file, syscall.O_APPEND | syscall.O_WRONLY | os.O_CREATE, nil},
		{file, syscall.O_APPEND | syscall.O_WRONLY | os.O_CREATE | os.O_TRUNC, nil},
		{file, syscall.O_WRONLY | syscall.O_RDWR, nil},
		{dir, syscall.O_WRONLY | syscall.O_RDWR, nil},
		{dir, syscall.O_RDONLY | syscall.O_TRUNC, syscall.ERROR_ACCESS_DENIED},
		{dir, syscall.O_WRONLY, syscall.EISDIR},
		{dir, syscall.O_RDWR, syscall.EISDIR},
	}
	for i, tt := range tests {
		h, err := syscall.Open(tt.path, tt.flag, 0o660)
		if err == nil {
			syscall.CloseHandle(h)
		}
		if err != tt.err {
			t.Errorf("%d: Open got %q, want %q", i, err, tt.err)
		}
	}
}

func TestComputerName(t *testing.T) {
	name, err := syscall.ComputerName()
	if err != nil {
		t.Fatalf("ComputerName failed: %v", err)
	}
	if len(name) == 0 {
		t.Error("ComputerName returned empty string")
	}
}

func TestWin32finddata(t *testing.T) {
	dir := t.TempDir()

	path := filepath.Join(dir, "long_name.and_extension")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create %v: %v", path, err)
	}
	f.Close()

	type X struct {
		fd  syscall.Win32finddata
		got byte
		pad [10]byte // to protect ourselves

	}
	var want byte = 2 // it is unlikely to have this character in the filename
	x := X{got: want}

	pathp, _ := syscall.UTF16PtrFromString(path)
	h, err := syscall.FindFirstFile(pathp, &(x.fd))
	if err != nil {
		t.Fatalf("FindFirstFile failed: %v", err)
	}
	err = syscall.FindClose(h)
	if err != nil {
		t.Fatalf("FindClose failed: %v", err)
	}

	if x.got != want {
		t.Fatalf("memory corruption: want=%d got=%d", want, x.got)
	}
}

func abort(funcname string, err error) {
	panic(funcname + " failed: " + err.Error())
}

func ExampleLoadLibrary() {
	h, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		abort("LoadLibrary", err)
	}
	defer syscall.FreeLibrary(h)
	proc, err := syscall.GetProcAddress(h, "GetVersion")
	if err != nil {
		abort("GetProcAddress", err)
	}
	r, _, _ := syscall.Syscall(uintptr(proc), 0, 0, 0, 0)
	major := byte(r)
	minor := uint8(r >> 8)
	build := uint16(r >> 16)
	print("windows version ", major, ".", minor, " (Build ", build, ")\n")
}

func TestTOKEN_ALL_ACCESS(t *testing.T) {
	if syscall.TOKEN_ALL_ACCESS != 0xF01FF {
		t.Errorf("TOKEN_ALL_ACCESS = %x, want 0xF01FF", syscall.TOKEN_ALL_ACCESS)
	}
}

func TestStdioAreInheritable(t *testing.T) {
	testenv.MustHaveGoBuild(t)
	testenv.MustHaveCGO(t)
	testenv.MustHaveExecPath(t, "gcc")

	tmpdir := t.TempDir()

	// build go dll
	const dlltext = `
package main

import "C"
import (
	"fmt"
)

//export HelloWorld
func HelloWorld() {
	fmt.Println("Hello World")
}

func main() {}
`
	dllsrc := filepath.Join(tmpdir, "helloworld.go")
	err := os.WriteFile(dllsrc, []byte(dlltext), 0644)
	if err != nil {
		t.Fatal(err)
	}
	dll := filepath.Join(tmpdir, "helloworld.dll")
	cmd := exec.Command(testenv.GoToolPath(t), "build", "-o", dll, "-buildmode", "c-shared", dllsrc)
	out, err := testenv.CleanCmdEnv(cmd).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build go library: %s\n%s", err, out)
	}

	// build c exe
	const exetext = `
#include <stdlib.h>
#include <windows.h>
int main(int argc, char *argv[])
{
	system("hostname");
	((void(*)(void))GetProcAddress(LoadLibraryA(%q), "HelloWorld"))();
	system("hostname");
	return 0;
}
`
	exe := filepath.Join(tmpdir, "helloworld.exe")
	cmd = exec.Command("gcc", "-o", exe, "-xc", "-")
	cmd.Stdin = strings.NewReader(fmt.Sprintf(exetext, dll))
	out, err = testenv.CleanCmdEnv(cmd).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build c executable: %s\n%s", err, out)
	}
	out, err = exec.Command(exe).Output()
	if err != nil {
		t.Fatalf("c program execution failed: %v: %v", err, string(out))
	}

	hostname, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}

	have := strings.ReplaceAll(string(out), "\n", "")
	have = strings.ReplaceAll(have, "\r", "")
	want := fmt.Sprintf("%sHello World%s", hostname, hostname)
	if have != want {
		t.Fatalf("c program output is wrong: got %q, want %q", have, want)
	}
}

func TestGetwd_DoesNotPanicWhenPathIsLong(t *testing.T) {
	// Regression test for https://github.com/golang/go/issues/60051.
	tmp := t.TempDir()
	t.Chdir(tmp)

	// The length of a filename is also limited, so we can't reproduce the
	// crash by creating a single directory with a very long name; we need two
	// layers.
	a200 := strings.Repeat("a", 200)
	dirname := filepath.Join(tmp, a200, a200)

	err := os.MkdirAll(dirname, 0o700)
	if err != nil {
		t.Skipf("MkdirAll failed: %v", err)
	}
	err = os.Chdir(dirname)
	if err != nil {
		t.Skipf("Chdir failed: %v", err)
	}

	syscall.Getwd()
}

func TestGetStartupInfo(t *testing.T) {
	var si syscall.StartupInfo
	err := syscall.GetStartupInfo(&si)
	if err != nil {
		// see https://go.dev/issue/31316
		t.Fatalf("GetStartupInfo: got error %v, want nil", err)
	}
}

// TestUtimesZeroLeavesTime checks that Utimes leaves alone each time it is
// given as zero and sets the other.
func TestUtimesZeroLeavesTime(t *testing.T) {
	name := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(name, nil, 0666); err != nil {
		t.Fatal(err)
	}
	const t1, t2 = 1600000000e9, 1700000000e9
	utimes := func(atime, mtime int64) {
		t.Helper()
		if err := syscall.Utimes(name, []syscall.Timeval{syscall.NsecToTimeval(atime), syscall.NsecToTimeval(mtime)}); err != nil {
			t.Fatal(err)
		}
	}
	wantMtime := func(want int64) {
		t.Helper()
		fi, err := os.Stat(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.ModTime().UnixNano(); got != want {
			t.Errorf("modification time = %d, want %d", got, want)
		}
	}
	utimes(t1, t1)
	wantMtime(t1)
	utimes(t2, 0)
	wantMtime(t1)
	utimes(0, t2)
	wantMtime(t2)
}

func TestSyscallAllocations(t *testing.T) {
	testenv.SkipIfOptimizationOff(t)

	// Test that syscall.SyscallN arguments do not escape.
	// The function used (in this case GetVersion) doesn't matter
	// as long as it is always available and doesn't panic.
	h, err := syscall.LoadLibrary("kernel32.dll")
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.FreeLibrary(h)
	proc, err := syscall.GetProcAddress(h, "GetVersion")
	if err != nil {
		t.Fatal(err)
	}

	testAllocs := func(t *testing.T, name string, fn func() error) {
		t.Run(name, func(t *testing.T) {
			n := int(testing.AllocsPerRun(10, func() {
				if err := fn(); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
			}))
			if n > 0 {
				t.Errorf("allocs = %d, want 0", n)
			}
		})
	}

	testAllocs(t, "SyscallN", func() error {
		r0, _, e1 := syscall.SyscallN(proc, 0, 0, 0)
		if r0 == 0 {
			return syscall.Errno(e1)
		}
		return nil
	})
	testAllocs(t, "Syscall", func() error {
		r0, _, e1 := syscall.Syscall(proc, 3, 0, 0, 0)
		if r0 == 0 {
			return syscall.Errno(e1)
		}
		return nil
	})
}

func FuzzUTF16FromString(f *testing.F) {
	f.Add("hi")           // ASCII
	f.Add("â")            // latin1
	f.Add("ねこ")           // plane 0
	f.Add("😃")            // extra Plane 0
	f.Add("\x90")         // invalid byte
	f.Add("\xe3\x81")     // truncated
	f.Add("\xe3\xc1\x81") // invalid middle byte

	f.Fuzz(func(t *testing.T, tst string) {
		res, err := syscall.UTF16FromString(tst)
		if err != nil {
			if strings.Contains(tst, "\x00") {
				t.Skipf("input %q contains a NUL byte", tst)
			}
			t.Fatalf("UTF16FromString(%q): %v", tst, err)
		}
		t.Logf("UTF16FromString(%q) = %04x", tst, res)

		if len(res) < 1 || res[len(res)-1] != 0 {
			t.Fatalf("missing NUL terminator")
		}
		if len(res) > len(tst)+1 {
			t.Fatalf("len(%04x) > len(%q)+1", res, tst)
		}
	})
}

func BenchmarkErrnoString(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = syscall.Errno(2).Error()
	}
}

// TestRtlGetNtVersionNumbers checks the version lookup StartProcess uses
// against RtlGetVersion.
func TestRtlGetNtVersionNumbers(t *testing.T) {
	var major, minor, build uint32
	syscall.RtlGetNtVersionNumbers(&major, &minor, &build)
	wantMajor, wantMinor, wantBuild := windows.Version()
	// The build number carries flag bits above its low 16 bits.
	got := [3]uint32{major, minor, build & 0xffff}
	want := [3]uint32{wantMajor, wantMinor, wantBuild}
	if got != want {
		t.Errorf("RtlGetNtVersionNumbers reports %v, RtlGetVersion reports %v", got, want)
	}
}

// TestOpenHoldingLinksNamedPipe checks that an open of a named pipe, by name
// or object path, with or without delete on close, takes one instance.
func TestOpenHoldingLinksNamedPipe(t *testing.T) {
	syscall.SetTestHoldLinks(true)
	t.Cleanup(func() { syscall.SetTestHoldLinks(false) })
	for i, c := range []struct {
		prefix string
		flag   int
	}{
		{`\\.\pipe\`, 0},
		{`\\.\pipe\`, windows.O_FILE_FLAG_DELETE_ON_CLOSE},
		{`\\?\GLOBALROOT\Device\NamedPipe\`, 0},
		{`\\?\GLOBALROOT\Device\NamedPipe\`, windows.O_FILE_FLAG_DELETE_ON_CLOSE},
	} {
		flag := c.flag
		pipe := fmt.Sprintf(`go-syscall-test-%d-%d`, os.Getpid(), i)
		namep, err := syscall.UTF16PtrFromString(`\\.\pipe\` + pipe)
		if err != nil {
			t.Fatal(err)
		}
		name := c.prefix + pipe
		for range 2 {
			server, err := windows.CreateNamedPipe(namep, windows.PIPE_ACCESS_DUPLEX, windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE, 2, 4096, 4096, 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer syscall.CloseHandle(server)
		}
		h, err := syscall.Open(name, syscall.O_RDONLY|syscall.O_CLOEXEC|flag, 0)
		if err != nil {
			t.Fatalf("Open of a named pipe holding links, flag %#x: %v", flag, err)
		}
		defer syscall.CloseHandle(h)
		h2, err := syscall.CreateFile(namep, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, 0, 0)
		if err != nil {
			t.Fatalf("second client of the two-instance pipe, flag %#x: %v, want the instance the first open left", flag, err)
		}
		defer syscall.CloseHandle(h2)
		const errPipeBusy = syscall.Errno(231)
		if _, err := syscall.CreateFile(namep, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, 0, 0); err != errPipeBusy {
			t.Errorf("third client of the two-instance pipe, flag %#x = %v, want %v", flag, err, errPipeBusy)
		}
	}
}

// TestOpenHoldingLinksDeleteOnClose checks that a delete-on-close open through
// a junction deletes nothing on the way through the link check.
func TestOpenHoldingLinksDeleteOnClose(t *testing.T) {
	// entries makes a file, a directory and a junction to each in a new
	// directory.
	entries := func() (file, sub, jf, jd string) {
		dir := t.TempDir()
		file = filepath.Join(dir, "file")
		if err := os.WriteFile(file, []byte("x"), 0666); err != nil {
			t.Fatal(err)
		}
		sub = filepath.Join(dir, "sub")
		if err := os.Mkdir(sub, 0777); err != nil {
			t.Fatal(err)
		}
		jf = filepath.Join(dir, "jf")
		jd = filepath.Join(dir, "jd")
		for _, l := range [][2]string{{jf, file}, {jd, sub}} {
			if out, err := exec.Command("cmd", "/c", "mklink", "/J", l[0], l[1]).CombinedOutput(); err != nil {
				t.Fatalf("mklink /J %s: %v\n%s", l[0], err, out)
			}
		}
		return
	}
	// left reports which of the link and its target are still there.
	left := func(link, target string) [2]bool {
		_, e1 := os.Lstat(link)
		_, e2 := os.Lstat(target)
		return [2]bool{e1 == nil, e2 == nil}
	}

	// What the kernel does with the open as asked, made directly.
	_, twinSub, _, twinJd := entries()
	p, err := syscall.UTF16PtrFromString(twinJd)
	if err != nil {
		t.Fatal(err)
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS|windows.O_FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err == nil {
		syscall.CloseHandle(h)
	}
	direct, directLeft := err, left(twinJd, twinSub)
	t.Logf("the open of a junction to a directory, made directly = %v, then junction left %v, directory left %v", direct, directLeft[0], directLeft[1])

	t.Cleanup(func() { syscall.SetTestHoldLinks(false) })
	for _, hold := range []bool{false, true} {
		syscall.SetTestHoldLinks(hold)
		file, sub, jf, jd := entries()
		const flag = syscall.O_RDONLY | syscall.O_CLOEXEC | windows.O_FILE_FLAG_DELETE_ON_CLOSE
		h, err := syscall.Open(jd, flag, 0)
		if err == nil {
			syscall.CloseHandle(h)
		}
		if err != direct {
			t.Errorf("Open(jd), holding links %v = %v, want %v as made directly", hold, err, direct)
		}
		if got := left(jd, sub); got != directLeft {
			t.Errorf("after Open(jd), holding links %v, junction and directory left %v, want %v as made directly", hold, got, directLeft)
		}
		if h, err := syscall.Open(jf, flag, 0); err != syscall.Errno(267) {
			if err == nil {
				syscall.CloseHandle(h)
			}
			t.Errorf("Open(jf), holding links %v = %v, want %v", hold, err, syscall.Errno(267))
		}
		for _, name := range []string{jf, file} {
			if _, err := os.Lstat(name); err != nil {
				t.Errorf("after Open(jf), holding links %v, %s: %v", hold, filepath.Base(name), err)
			}
		}
		// An open that may create is refused the same way, with nothing
		// deleted.
		if h, err := syscall.Open(jf, flag|syscall.O_CREAT, 0666); err != syscall.Errno(267) {
			if err == nil {
				syscall.CloseHandle(h)
			}
			t.Errorf("Open(jf, O_CREAT), holding links %v = %v, want %v", hold, err, syscall.Errno(267))
		}
		for _, name := range []string{jf, file} {
			if _, err := os.Lstat(name); err != nil {
				t.Errorf("after Open(jf, O_CREAT), holding links %v, %s: %v", hold, filepath.Base(name), err)
			}
		}
	}
}

// TestIsPipeName checks which names Open treats as named pipes.
func TestIsPipeName(t *testing.T) {
	for name, want := range map[string]bool{
		`\\.\pipe\p`:                        true,
		`\\?\pipe\p`:                        true,
		`//./PIPE/p`:                        true,
		`\\.\Pipe\`:                         true,
		`\\server\pipe\p`:                   true,
		`\\?\UNC\server\pipe\p`:             true,
		`\\?\unc\server\PIPE\p`:             true,
		`\\.\pipe`:                          false,
		`\\.\pipes\p`:                       false,
		`\\.\C:\pipe\p`:                     false,
		`\\?\C:\pipe\p`:                     false,
		`\\server\share\pipe\p`:             false,
		`\\server\pipe`:                     false,
		`\\\pipe\p`:                         false,
		`C:\pipe\p`:                         false,
		`\\?\GLOBALROOT\Device\NamedPipe\p`: false,
		`pipe\p`:                            false,
		``:                                  false,
	} {
		if got := syscall.IsPipeName(name); got != want {
			t.Errorf("isPipeName(%q) = %v, want %v", name, got, want)
		}
	}
}
