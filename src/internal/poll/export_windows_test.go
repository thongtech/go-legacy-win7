// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Export guts for testing on windows.

package poll

func SkipsCompletionPortOnSuccess(fd *FD) bool {
	return fd.skipSyncNotif
}

var (
	IsStreamSocket               = isStreamSocket
	TruncatedRecvSkipsCompletion = truncatedRecvSkipsCompletion
	AcceptCompletionNeedsThread  = acceptCompletionNeedsThread
)

// AcceptThreadID is the OS thread the accept thread runs on, or zero if it
// has not been started.
func AcceptThreadID() uint32 {
	return acceptThreadID.Load()
}

// AcceptThreadPosts is the number of AcceptEx calls posted from the accept
// thread so far.
func AcceptThreadPosts() uint64 {
	return acceptThreadPosts.Load()
}
