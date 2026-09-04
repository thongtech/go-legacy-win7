// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"internal/poll"
	"io"
	"testing"
	"time"
)

// TestFileNoDisassociateIOCP covers the File methods on a kernel that cannot
// detach a handle from its completion port. The fallback is forced on so that
// it is covered on every Windows version.
func TestFileNoDisassociateIOCP(t *testing.T) {
	poll.TestDisassociateIOCPUnsupported = true
	t.Cleanup(func() { poll.TestDisassociateIOCPUnsupported = false })

	ln := newLocalListener(t, "tcp")
	defer ln.Close()

	c, err := Dial(ln.Addr().Network(), ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	server, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	f, err := c.(*TCPConn).File()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dup, err := FileConn(f)
	if err != nil {
		t.Fatal(err)
	}
	defer dup.Close()
	if _, err := dup.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 5)
	if _, err := io.ReadFull(server, b); err != nil {
		t.Fatal(err)
	}
	if got := string(b); got != "hello" {
		t.Errorf("read %q from the duplicated connection, want %q", got, "hello")
	}

	lf, err := ln.(*TCPListener).File()
	if err != nil {
		t.Fatal(err)
	}
	defer lf.Close()
	ln2, err := FileListener(lf)
	if err != nil {
		t.Fatal(err)
	}
	defer ln2.Close()
	c2, err := Dial(ln2.Addr().Network(), ln2.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	accepted, err := ln2.Accept()
	if err != nil {
		t.Fatal(err)
	}
	accepted.Close()
}

// TestFileNoDisassociateIOCPClose checks that Close ends a Read on a
// connection the poller could not adopt. Nothing but Close can, because the
// poller is not watching the handle and the handle cannot close while the
// Read holds it.
func TestFileNoDisassociateIOCPClose(t *testing.T) {
	poll.TestDisassociateIOCPUnsupported = true
	t.Cleanup(func() { poll.TestDisassociateIOCPUnsupported = false })

	ln := newLocalListener(t, "tcp")
	defer ln.Close()
	c, err := Dial(ln.Addr().Network(), ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	server, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	f, err := c.(*TCPConn).File()
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dup, err := FileConn(f)
	if err != nil {
		t.Fatal(err)
	}

	read := make(chan error, 1)
	go func() {
		_, err := dup.Read(make([]byte, 1))
		read <- err
	}()
	// Let the Read get as far as waiting. If it has not, Close still has to
	// refuse it, and either way it must come back.
	time.Sleep(50 * time.Millisecond)
	closed := make(chan error, 1)
	go func() { closed <- dup.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Errorf("Close = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not return while a Read was pending")
	}
	select {
	case err := <-read:
		if err == nil {
			t.Error("Read returned no error after Close")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Read did not return after Close")
	}
}
