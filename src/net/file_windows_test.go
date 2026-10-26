// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"internal/poll"
	"testing"
	"time"
)

// TestFileNoDisassociateIOCPClose checks that Close ends a Read on a
// connection the poller could not adopt.
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
