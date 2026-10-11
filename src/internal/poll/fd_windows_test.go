// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package poll_test

import (
	"errors"
	"fmt"
	"internal/poll"
	"internal/syscall/windows"
	"internal/testenv"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"testing/synctest"
	"time"
	"unsafe"
)

func init() {
	poll.InitWSA()
}

func TestWSASocketConflict(t *testing.T) {
	t.Parallel()
	s, err := windows.WSASocket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED)
	if err != nil {
		t.Fatal(err)
	}
	fd := poll.FD{Sysfd: s, IsStream: true, ZeroReadIsEOF: true}
	if err = fd.Init("tcp", true); err != nil {
		syscall.CloseHandle(s)
		t.Fatal(err)
	}
	defer fd.Close()

	const SIO_TCP_INFO = syscall.IOC_INOUT | syscall.IOC_VENDOR | 39
	inbuf := uint32(0)
	var outbuf _TCP_INFO_v0
	cbbr := uint32(0)

	var ov syscall.Overlapped
	// Create an event so that we can efficiently wait for completion
	// of a requested overlapped I/O operation.
	ov.HEvent, _ = windows.CreateEvent(nil, 0, 0, nil)
	if ov.HEvent == 0 {
		t.Fatalf("could not create the event!")
	}
	defer syscall.CloseHandle(ov.HEvent)

	if err = fd.WSAIoctl(
		SIO_TCP_INFO,
		(*byte)(unsafe.Pointer(&inbuf)),
		uint32(unsafe.Sizeof(inbuf)),
		(*byte)(unsafe.Pointer(&outbuf)),
		uint32(unsafe.Sizeof(outbuf)),
		&cbbr,
		&ov,
		0,
	); err != nil && !errors.Is(err, syscall.ERROR_IO_PENDING) {
		t.Fatalf("could not perform the WSAIoctl: %v", err)
	}

	if err != nil && errors.Is(err, syscall.ERROR_IO_PENDING) {
		// It is possible that the overlapped I/O operation completed
		// immediately so there is no need to wait for it to complete.
		if res, err := syscall.WaitForSingleObject(ov.HEvent, syscall.INFINITE); res != 0 {
			t.Fatalf("waiting for the completion of the overlapped IO failed: %v", err)
		}
	}
}

type _TCP_INFO_v0 struct {
	State             uint32
	Mss               uint32
	ConnectionTimeMs  uint64
	TimestampsEnabled bool
	RttUs             uint32
	MinRttUs          uint32
	BytesInFlight     uint32
	Cwnd              uint32
	SndWnd            uint32
	RcvWnd            uint32
	RcvBuf            uint32
	BytesOut          uint64
	BytesIn           uint64
	BytesReordered    uint32
	BytesRetrans      uint32
	FastRetrans       uint32
	DupAcksIn         uint32
	TimeoutEpisodes   uint32
	SynRetrans        uint8
}

func newFD(t testing.TB, h syscall.Handle, kind string, overlapped bool) *poll.FD {
	fd := poll.FD{
		Sysfd:         h,
		IsStream:      true,
		ZeroReadIsEOF: true,
	}
	err := fd.Init(kind, overlapped)
	if overlapped && err != nil {
		// Overlapped file handles should not error.
		fd.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fd.Close()
	})
	return &fd
}

func newFile(t testing.TB, name string, overlapped bool) *poll.FD {
	namep, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	flags := syscall.FILE_ATTRIBUTE_NORMAL
	if overlapped {
		flags |= syscall.FILE_FLAG_OVERLAPPED
	}
	h, err := syscall.CreateFile(namep,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_READ,
		nil, syscall.OPEN_ALWAYS, uint32(flags), 0)
	if err != nil {
		t.Fatal(err)
	}
	typ, err := syscall.GetFileType(h)
	if err != nil {
		syscall.CloseHandle(h)
		t.Fatal(err)
	}
	kind := "file"
	if typ == syscall.FILE_TYPE_PIPE {
		kind = "pipe"
	}
	return newFD(t, h, kind, overlapped)
}

func TestFileSkipsCompletionPortOnSuccess(t *testing.T) {
	t.Parallel()
	fd := newFile(t, filepath.Join(t.TempDir(), "foo"), true)
	if !poll.SkipsCompletionPortOnSuccess(fd) {
		t.Fatal("expected file handles to skip completion port on success")
	}
}

func TestSocketSkipsCompletionPortOnSuccess(t *testing.T) {
	// Assume that all Windows we test on only have IFS handles for TCP sockets.
	t.Parallel()
	s, err := windows.WSASocket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED)
	if err != nil {
		t.Fatal(err)
	}
	fd := newFD(t, s, "tcp", true)
	if !poll.SkipsCompletionPortOnSuccess(fd) {
		t.Fatal("expected socket handles to skip completion port on success")
	}
}

func TestIsStreamSocket(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		typ   int32
		proto int32
		want  bool
	}{
		{"tcp", syscall.SOCK_STREAM, syscall.IPPROTO_TCP, true},
		{"udp", syscall.SOCK_DGRAM, syscall.IPPROTO_UDP, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, err := windows.WSASocket(syscall.AF_INET, tt.typ, tt.proto, nil, 0, windows.WSA_FLAG_OVERLAPPED)
			if err != nil {
				t.Fatal(err)
			}
			defer syscall.Closesocket(s)
			if got := poll.IsStreamSocket(s); got != tt.want {
				t.Errorf("IsStreamSocket(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// TestDatagramSocketSkipsCompletionPortOnSuccess covers the receive that hung
// net.TestReadFromTimeout on Windows 7. The kernel behaviour cannot be
// provoked elsewhere, so what is asserted is the decision FD.Init reaches.
func TestDatagramSocketSkipsCompletionPortOnSuccess(t *testing.T) {
	// Assume that all Windows we test on only have IFS handles for UDP sockets.
	t.Parallel()
	s, err := windows.WSASocket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP, nil, 0, windows.WSA_FLAG_OVERLAPPED)
	if err != nil {
		t.Fatal(err)
	}
	fd := newFD(t, s, "udp", true)
	want := !poll.TruncatedRecvSkipsCompletion()
	if got := poll.SkipsCompletionPortOnSuccess(fd); got != want {
		t.Errorf("datagram socket skips completion port on success = %v, want %v", got, want)
	}
}

// acceptOnce listens on a loopback TCP socket, connects a client to it and
// accepts that connection through FD.Accept from the calling goroutine.
func acceptOnce(t *testing.T) {
	t.Helper()
	ls, err := windows.WSASocket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Bind(ls, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Listen(ls, 1); err != nil {
		t.Fatal(err)
	}
	sa, err := syscall.Getsockname(ls)
	if err != nil {
		t.Fatal(err)
	}
	fd := newFD(t, ls, "tcp", true)

	c, err := windows.WSASocket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Closesocket(c)
	connected := make(chan error, 1)
	go func() {
		connected <- syscall.Connect(c, &syscall.SockaddrInet4{Port: sa.(*syscall.SockaddrInet4).Port, Addr: [4]byte{127, 0, 0, 1}})
	}()

	s, _, _, _, err := fd.Accept(func() (syscall.Handle, error) {
		return windows.WSASocket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED)
	})
	if err != nil {
		t.Fatal(err)
	}
	syscall.Closesocket(s)
	if err := <-connected; err != nil {
		t.Fatal(err)
	}
}

// TestAcceptPostedFromAcceptThread covers the accept thread, which on Windows
// 8 and Server 2012 keeps an AcceptEx from being posted by a thread that may
// later block in a synchronous read. The kernel behaviour cannot be provoked
// elsewhere, so the fallback is forced on and what is asserted is where the
// AcceptEx was posted from.
func TestAcceptPostedFromAcceptThread(t *testing.T) {
	poll.TestUseAcceptThread = true
	t.Cleanup(func() { poll.TestUseAcceptThread = false })

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	self := windows.GetCurrentThreadId()

	before := poll.AcceptThreadPosts()
	acceptOnce(t)

	thread := poll.AcceptThreadID()
	if thread == 0 {
		t.Fatal("the accept thread was not started")
	}
	if thread == self {
		t.Fatal("the accept thread is the thread of the goroutine that accepted")
	}
	if got := poll.AcceptThreadPosts() - before; got != 1 {
		t.Errorf("%d AcceptEx calls were posted from the accept thread, want 1", got)
	}
}

// TestAcceptThreadInSynctestBubble checks that an accept made inside a
// synctest bubble goes through the accept thread without the bubble taking
// anything of the thread's for its own. A goroutine the bubble owned would
// be reported as a deadlock when the bubble ends, and a channel it owned
// could not be replied on from outside.
func TestAcceptThreadInSynctestBubble(t *testing.T) {
	poll.TestUseAcceptThread = true
	t.Cleanup(func() { poll.TestUseAcceptThread = false })

	before := poll.AcceptThreadPosts()
	synctest.Test(t, func(t *testing.T) {
		acceptOnce(t)
	})
	if got := poll.AcceptThreadPosts() - before; got != 1 {
		t.Errorf("%d AcceptEx calls were posted from the accept thread, want 1", got)
	}
}

// TestAcceptThreadIsNotAGoroutine checks that the accept thread leaves no
// goroutine behind between posts. One that stayed would count in
// runtime.NumGoroutine and change what the runtime reports for a deadlock,
// in every program on the kernel that needs the thread.
func TestAcceptThreadIsNotAGoroutine(t *testing.T) {
	poll.TestUseAcceptThread = true
	t.Cleanup(func() { poll.TestUseAcceptThread = false })

	before := runtime.NumGoroutine()
	acceptOnce(t)
	// The client goroutine acceptOnce started may still be on its way out,
	// and so may goroutines of earlier tests, which is why fewer is fine.
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := runtime.NumGoroutine(); got > before {
		t.Errorf("NumGoroutine = %d after an accept, want at most %d as before", got, before)
	}
}

// TestMain runs acceptAlone instead of the tests when asked to, for
// TestAcceptThreadKeepsProgramAlive.
func TestMain(m *testing.M) {
	if os.Getenv("GO_POLL_TEST_ACCEPT_ALONE") == "1" {
		acceptAlone()
		return
	}
	os.Exit(m.Run())
}

// acceptAlone accepts one connection through the accept thread in a program
// whose only goroutine is the one accepting, with the connection already
// made, so that nothing else keeps the runtime's deadlock check quiet. It
// exits with a message when the accept fails, and dies with the runtime's
// "all goroutines are asleep" when the check fires.
func acceptAlone() {
	poll.TestUseAcceptThread = true
	ls, err := windows.WSASocket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED)
	if err != nil {
		fmt.Println("socket:", err)
		os.Exit(1)
	}
	if err := syscall.Bind(ls, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		fmt.Println("bind:", err)
		os.Exit(1)
	}
	if err := syscall.Listen(ls, 1); err != nil {
		fmt.Println("listen:", err)
		os.Exit(1)
	}
	sa, err := syscall.Getsockname(ls)
	if err != nil {
		fmt.Println("getsockname:", err)
		os.Exit(1)
	}
	fd := poll.FD{Sysfd: ls, IsStream: true, ZeroReadIsEOF: true}
	if err := fd.Init("tcp", true); err != nil {
		fmt.Println("init:", err)
		os.Exit(1)
	}
	c, err := windows.WSASocket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED)
	if err != nil {
		fmt.Println("socket:", err)
		os.Exit(1)
	}
	if err := syscall.Connect(c, &syscall.SockaddrInet4{Port: sa.(*syscall.SockaddrInet4).Port, Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		fmt.Println("connect:", err)
		os.Exit(1)
	}
	s, _, _, _, err := fd.Accept(func() (syscall.Handle, error) {
		return windows.WSASocket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED)
	})
	if err != nil {
		fmt.Println("accept:", err)
		os.Exit(1)
	}
	syscall.Closesocket(s)
	syscall.Closesocket(c)
	fd.Close()
}

// TestAcceptThreadKeepsProgramAlive checks that a program whose only
// goroutine is waiting for the accept thread is not taken for a deadlock. The
// runtime's check does not know that the pool thread will call back, so the
// caller has to wait in a system call, which counts as running, and not on a
// channel. A test binary always has a timer goroutine, which keeps the check
// quiet, so the case needs a process of its own.
func TestAcceptThreadKeepsProgramAlive(t *testing.T) {
	cmd := testenv.Command(t, testenv.Executable(t))
	cmd.Env = append(cmd.Environ(), "GO_POLL_TEST_ACCEPT_ALONE=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the program did not accept and exit: %v\n%s", err, out)
	}
}

// TestAcceptPostedInline checks that where the kernel does not need the
// accept thread, an AcceptEx is posted from the thread that asked for it and
// the accept thread is left alone.
func TestAcceptPostedInline(t *testing.T) {
	if poll.AcceptCompletionNeedsThread() {
		t.Skip("this kernel posts every AcceptEx from the accept thread")
	}
	before := poll.AcceptThreadPosts()
	acceptOnce(t)
	if got := poll.AcceptThreadPosts() - before; got != 0 {
		t.Errorf("%d AcceptEx calls were posted from the accept thread, want none", got)
	}
}

func BenchmarkReadOverlapped(b *testing.B) {
	benchmarkRead(b, true)
}

func BenchmarkReadSync(b *testing.B) {
	benchmarkRead(b, false)
}

func benchmarkRead(b *testing.B, overlapped bool) {
	name := filepath.Join(b.TempDir(), "foo")
	const content = "hello world"
	err := os.WriteFile(name, []byte(content), 0644)
	if err != nil {
		b.Fatal(err)
	}
	file := newFile(b, name, overlapped)
	var buf [len(content)]byte
	for b.Loop() {
		_, err := io.ReadFull(file, buf[:])
		if err != nil {
			b.Fatal(err)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			b.Fatal(err)
		}
	}
}
