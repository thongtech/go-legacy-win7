// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package os_test

import (
	"errors"
	"fmt"
	"internal/syscall/windows"
	"internal/testenv"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestAddExtendedPrefix(t *testing.T) {
	// Test addExtendedPrefix instead of fixLongPath so the path manipulation code
	// is exercised even if long path are supported by the system, else the
	// function might not be tested at all if/when all test builders support long paths.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal("cannot get cwd")
	}
	drive := strings.ToLower(filepath.VolumeName(cwd))
	cwd = strings.ToLower(cwd[len(drive)+1:])
	// Build a very long pathname. Paths in Go are supposed to be arbitrarily long,
	// so let's make a long path which is comfortably bigger than MAX_PATH on Windows
	// (256) and thus requires fixLongPath to be correctly interpreted in I/O syscalls.
	veryLong := "l" + strings.Repeat("o", 500) + "ng"
	for _, test := range []struct{ in, want string }{
		// Test cases use word substitutions:
		//   * "long" is replaced with a very long pathname
		//   * "c:" or "C:" are replaced with the drive of the current directory (preserving case)
		//   * "cwd" is replaced with the current directory

		// Drive Absolute
		{`C:\long\foo.txt`, `\\?\C:\long\foo.txt`},
		{`C:/long/foo.txt`, `\\?\C:\long\foo.txt`},
		{`C:\\\long///foo.txt`, `\\?\C:\long\foo.txt`},
		{`C:\long\.\foo.txt`, `\\?\C:\long\foo.txt`},
		{`C:\long\..\foo.txt`, `\\?\C:\foo.txt`},
		{`C:\long\..\..\foo.txt`, `\\?\C:\foo.txt`},

		// Drive Relative
		{`C:long\foo.txt`, `\\?\C:\cwd\long\foo.txt`},
		{`C:long/foo.txt`, `\\?\C:\cwd\long\foo.txt`},
		{`C:long///foo.txt`, `\\?\C:\cwd\long\foo.txt`},
		{`C:long\.\foo.txt`, `\\?\C:\cwd\long\foo.txt`},
		{`C:long\..\foo.txt`, `\\?\C:\cwd\foo.txt`},

		// Rooted
		{`\long\foo.txt`, `\\?\C:\long\foo.txt`},
		{`/long/foo.txt`, `\\?\C:\long\foo.txt`},
		{`\long///foo.txt`, `\\?\C:\long\foo.txt`},
		{`\long\.\foo.txt`, `\\?\C:\long\foo.txt`},
		{`\long\..\foo.txt`, `\\?\C:\foo.txt`},

		// Relative
		{`long\foo.txt`, `\\?\C:\cwd\long\foo.txt`},
		{`long/foo.txt`, `\\?\C:\cwd\long\foo.txt`},
		{`long///foo.txt`, `\\?\C:\cwd\long\foo.txt`},
		{`long\.\foo.txt`, `\\?\C:\cwd\long\foo.txt`},
		{`long\..\foo.txt`, `\\?\C:\cwd\foo.txt`},
		{`.\long\foo.txt`, `\\?\C:\cwd\long\foo.txt`},

		// UNC Absolute
		{`\\srv\share\long`, `\\?\UNC\srv\share\long`},
		{`//srv/share/long`, `\\?\UNC\srv\share\long`},
		{`/\srv/share/long`, `\\?\UNC\srv\share\long`},
		{`\\srv\share\long\`, `\\?\UNC\srv\share\long\`},
		{`\\srv\share\bar\.\long`, `\\?\UNC\srv\share\bar\long`},
		{`\\srv\share\bar\..\long`, `\\?\UNC\srv\share\long`},
		{`\\srv\share\bar\..\..\long`, `\\?\UNC\srv\share\long`}, // share name is not removed by ".."

		// Local Device
		{`\\.\C:\long\foo.txt`, `\\.\C:\long\foo.txt`},
		{`//./C:/long/foo.txt`, `\\.\C:\long\foo.txt`},
		{`/\./C:/long/foo.txt`, `\\.\C:\long\foo.txt`},
		{`\\.\C:\long///foo.txt`, `\\.\C:\long\foo.txt`},
		{`\\.\C:\long\.\foo.txt`, `\\.\C:\long\foo.txt`},
		{`\\.\C:\long\..\foo.txt`, `\\.\C:\foo.txt`},

		// Misc tests
		{`C:\short.txt`, `C:\short.txt`},
		{`C:\`, `C:\`},
		{`C:`, `C:`},
		{`\\srv\path`, `\\srv\path`},
		{`long.txt`, `\\?\C:\cwd\long.txt`},
		{`C:long.txt`, `\\?\C:\cwd\long.txt`},
		{`C:\long\.\bar\baz`, `\\?\C:\long\bar\baz`},
		{`C:long\.\bar\baz`, `\\?\C:\cwd\long\bar\baz`},
		{`C:\long\..\bar\baz`, `\\?\C:\bar\baz`},
		{`C:long\..\bar\baz`, `\\?\C:\cwd\bar\baz`},
		{`C:\long\foo\\bar\.\baz\\`, `\\?\C:\long\foo\bar\baz\`},
		{`C:\long\..`, `\\?\C:\`},
		{`C:\.\long\..\.`, `\\?\C:\`},
		{`\\?\C:\long\foo.txt`, `\\?\C:\long\foo.txt`},
		{`\\?\C:\long/foo.txt`, `\\?\C:\long/foo.txt`},
	} {
		in := strings.ReplaceAll(test.in, "long", veryLong)
		in = strings.ToLower(in)
		in = strings.ReplaceAll(in, "c:", drive)

		want := strings.ReplaceAll(test.want, "long", veryLong)
		want = strings.ToLower(want)
		want = strings.ReplaceAll(want, "c:", drive)
		want = strings.ReplaceAll(want, "cwd", cwd)

		got := os.AddExtendedPrefix(in)
		got = strings.ToLower(got)
		if got != want {
			in = strings.ReplaceAll(in, veryLong, "long")
			got = strings.ReplaceAll(got, veryLong, "long")
			want = strings.ReplaceAll(want, veryLong, "long")
			t.Errorf("addExtendedPrefix(%#q) = %#q; want %#q", in, got, want)
		}
	}
}

func TestMkdirAllLongPath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := tmpDir
	for i := 0; i < 100; i++ {
		path += `\another-path-component`
	}
	if err := os.MkdirAll(path, 0777); err != nil {
		t.Fatalf("MkdirAll(%q) failed; %v", path, err)
	}
	if err := os.RemoveAll(tmpDir); err != nil {
		t.Fatalf("RemoveAll(%q) failed; %v", tmpDir, err)
	}
}

func TestMkdirAllExtendedLength(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	const prefix = `\\?\`
	if len(tmpDir) < 4 || tmpDir[:4] != prefix {
		fullPath, err := syscall.FullPath(tmpDir)
		if err != nil {
			t.Fatalf("FullPath(%q) fails: %v", tmpDir, err)
		}
		tmpDir = prefix + fullPath
	}
	path := tmpDir + `\dir\`
	if err := os.MkdirAll(path, 0777); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", path, err)
	}

	path = path + `.\dir2`
	if err := os.MkdirAll(path, 0777); err == nil {
		t.Fatalf("MkdirAll(%q) should have failed, but did not", path)
	}
}

func TestOpenRootSlash(t *testing.T) {
	t.Parallel()

	tests := []string{
		`/`,
		`\`,
	}

	for _, test := range tests {
		dir, err := os.Open(test)
		if err != nil {
			t.Fatalf("Open(%q) failed: %v", test, err)
		}
		dir.Close()
	}
}

func testMkdirAllAtRoot(t *testing.T, root string) {
	// Create a unique-enough directory name in root.
	base := fmt.Sprintf("%s-%d", t.Name(), os.Getpid())
	path := filepath.Join(root, base)
	if err := os.MkdirAll(path, 0777); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", path, err)
	}
	// Clean up
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
}

func TestMkdirAllExtendedLengthAtRoot(t *testing.T) {
	if testenv.Builder() == "" {
		t.Skipf("skipping non-hermetic test outside of Go builders")
	}

	const prefix = `\\?\`
	vol := filepath.VolumeName(t.TempDir()) + `\`
	if len(vol) < 4 || vol[:4] != prefix {
		vol = prefix + vol
	}
	testMkdirAllAtRoot(t, vol)
}

func TestMkdirAllVolumeNameAtRoot(t *testing.T) {
	if testenv.Builder() == "" {
		t.Skipf("skipping non-hermetic test outside of Go builders")
	}

	vol, err := syscall.UTF16PtrFromString(filepath.VolumeName(t.TempDir()) + `\`)
	if err != nil {
		t.Fatal(err)
	}
	const maxVolNameLen = 50
	var buf [maxVolNameLen]uint16
	err = windows.GetVolumeNameForVolumeMountPoint(vol, &buf[0], maxVolNameLen)
	if err != nil {
		t.Fatal(err)
	}
	volName := syscall.UTF16ToString(buf[:])
	testMkdirAllAtRoot(t, volName)
}

func TestRemoveAllLongPathRelative(t *testing.T) {
	// Test that RemoveAll doesn't hang with long relative paths.
	// See go.dev/issue/36375.
	tmp := t.TempDir()
	t.Chdir(tmp)
	dir := filepath.Join(tmp, "foo", "bar", strings.Repeat("a", 150), strings.Repeat("b", 150))
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatal(err)
	}
	err = os.RemoveAll("foo")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRemoveAllFallback(t *testing.T) {
	windows.TestDeleteatFallback = true
	t.Cleanup(func() { windows.TestDeleteatFallback = false })

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file1"), []byte{}, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file2"), []byte{}, 0400); err != nil { // read-only file
		t.Fatal(err)
	}
	// A read-only directory, which the read-only attribute has to be cleared
	// from just as it does for a file. Windows 10, version 1809 asks the
	// kernel to ignore the attribute instead, so only the fallback meets this.
	sub := filepath.Join(dir, "dir")
	if err := os.Mkdir(sub, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file3"), []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sub, 0555); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
}

// TestRemoveFallbackOpenFile checks that a file removed while it is still open
// gives its name up at once, through Root.Remove and through os.Remove. Only
// the fallback is covered. Windows 10, version 1607 and later ask the kernel
// for POSIX semantics and never reach it, and there is no way to ask a kernel
// that lacks them to behave as though it had them. DeleteFileW frees the name
// on its own there as well, so the staging directory is also checked for the
// moved copy, which is what shows that the fallback ran at all.
func TestRemoveFallbackOpenFile(t *testing.T) {
	windows.TestDeleteatFallback = true
	t.Cleanup(func() { windows.TestDeleteatFallback = false })

	// movedCopy reports whether the staging directory, the temporary
	// directory on the same volume as t.TempDir, lists the copy of the file
	// open on f, which the fallback names after the file's identifier. The
	// directory is listed rather than the copy looked up, because a file
	// marked for deletion can still be listed where it cannot be opened.
	movedCopy := func(f *os.File) bool {
		var info syscall.ByHandleFileInformation
		if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &info); err != nil {
			t.Fatal(err)
		}
		want := fmt.Sprintf(".go-deleted-%08x%08x", info.FileIndexHigh, info.FileIndexLow)
		entries, err := os.ReadDir(os.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() == want {
				return true
			}
		}
		return false
	}

	for _, tt := range []struct {
		name   string
		remove func(root *os.Root, path string) error
	}{
		{"Root", func(root *os.Root, path string) error { return root.Remove(filepath.Base(path)) }},
		{"os", func(root *os.Root, path string) error { return os.Remove(path) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			name := filepath.Join(dir, "file")
			if err := os.WriteFile(name, []byte("old"), 0600); err != nil {
				t.Fatal(err)
			}

			// Hold the file open across the removal, the way a leaked handle
			// would. Root.Open shares deletion, which is what lets the file
			// be removed while the handle is still there.
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			f, err := root.Open("file")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			if err := tt.remove(root, name); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(name); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("Lstat after Remove = %v, want ErrNotExist", err)
			}
			if !movedCopy(f) {
				t.Error("the moved copy is not in the staging directory, so the fallback did not run")
			}
			// The name is free, so it can be taken again.
			if err := os.WriteFile(name, []byte("new"), 0600); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != "new" {
				t.Errorf("read %q, want %q", b, "new")
			}
		})
	}
}

// TestRemoveAllFallbackOpenFile checks that a tree comes down while a file in
// it is still open. See TestRemoveFallbackOpenFile on why only the fallback is
// covered.
func TestRemoveAllFallbackOpenFile(t *testing.T) {
	windows.TestDeleteatFallback = true
	t.Cleanup(func() { windows.TestDeleteatFallback = false })

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	f, err := root.Open("sub/file")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := os.RemoveAll(sub); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sub); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat after RemoveAll = %v, want ErrNotExist", err)
	}
}

// TestOpenatNoObjDontReparse covers Openat on a kernel that rejects
// OBJ_DONT_REPARSE, which Windows 7 does. The O_NOFOLLOW_ANY guarantee
// must survive the fallback, because RemoveAll walks a tree with it and
// following a symlink there recurses out of the tree.
func TestOpenatNoObjDontReparse(t *testing.T) {
	windows.TestOpenatNoObjDontReparse = true
	t.Cleanup(func() { windows.TestOpenatNoObjDontReparse = false })

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(filepath.Join(sub, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "file"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	f, err := root.Open("sub/file")
	if err != nil {
		t.Fatalf("Root.Open: %v", err)
	}
	f.Close()

	// A symlink must still be refused rather than followed.
	if testenv.HasSymlink() {
		if err := os.Symlink(sub, filepath.Join(dir, "link")); err != nil {
			t.Fatal(err)
		}
		if _, err := root.Open("link/file"); err == nil {
			t.Error("Root.Open followed a symlink, want error")
		}
	}

	// Windows will not remove a directory with an open handle on it.
	root.Close()

	// This is the operation that failed on Windows 7.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
}

// TestOpenatMalformedKeepsObjDontReparse checks that an open refused for a
// reason of its own, a trailing separator on a file, is not mistaken for a
// kernel that rejects OBJ_DONT_REPARSE.
func TestOpenatMalformedKeepsObjDontReparse(t *testing.T) {
	if windows.ObjDontReparseUnsupportedForTest() {
		t.Skip("kernel already known to reject OBJ_DONT_REPARSE")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "target"), []byte("target"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	// A trailing separator on a file. This has to fail, and what matters is
	// what it leaves behind.
	if f, err := root.Create("target/"); err == nil {
		f.Close()
		t.Fatal(`Root.Create("target/") succeeded, want an error`)
	}
	if windows.ObjDontReparseUnsupportedForTest() {
		t.Error("a malformed open was taken for a kernel without OBJ_DONT_REPARSE")
	}
}

func testLongPathAbs(t *testing.T, target string) {
	t.Helper()
	testWalkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Error(err)
		}
		return err
	}
	if err := os.MkdirAll(target, 0777); err != nil {
		t.Fatal(err)
	}
	// Test that Walk doesn't fail with long paths.
	// See go.dev/issue/21782.
	filepath.Walk(target, testWalkFn)
	// Test that RemoveAll doesn't hang with long paths.
	// See go.dev/issue/36375.
	if err := os.RemoveAll(target); err != nil {
		t.Error(err)
	}
}

func TestLongPathAbs(t *testing.T) {
	t.Parallel()

	target := t.TempDir() + "\\" + strings.Repeat("a\\", 300)
	testLongPathAbs(t, target)
}

func TestLongPathRel(t *testing.T) {
	t.Chdir(t.TempDir())

	target := strings.Repeat("b\\", 300)
	testLongPathAbs(t, target)
}

func BenchmarkAddExtendedPrefix(b *testing.B) {
	veryLong := `C:\l` + strings.Repeat("o", 248) + "ng"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		os.AddExtendedPrefix(veryLong)
	}
}

// TestRenameatDirOntoLinkKeepsLink covers renaming a directory that has an
// open file beneath it onto a link. The kernel refuses that for the open file,
// not for the link, and the link has to survive it. The fallback is forced on
// as in TestRenameatNoPosixSemantics.
func TestRenameatDirOntoLinkKeepsLink(t *testing.T) {
	output, _ := testenv.Command(t, "cmd", "/c", "mklink", "/?").Output()
	if !strings.Contains(string(output), " /J ") {
		t.Skip("skipping test because mklink command does not support junctions")
	}
	windows.TestRenameatNoPosixSemantics = true
	t.Cleanup(func() { windows.TestRenameatNoPosixSemantics = false })

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if out, err := testenv.Command(t, "cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J %v %v: %v\n%s", link, target, err, out)
	}
	if err := os.Mkdir(filepath.Join(dir, "source"), 0777); err != nil {
		t.Fatal(err)
	}
	held, err := os.Create(filepath.Join(dir, "source", "held"))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.Rename("source", "link"); err == nil {
		t.Fatal("Rename of a directory with an open file beneath it succeeded")
	}
	// The link is still there and still leads to target, not to source.
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("link after the refused rename: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(link, "held")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat(link/held) = %v, want ErrNotExist", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "source", "held")); err != nil {
		t.Errorf("source/held after the refused rename: %v", err)
	}
}

// TestRenameatNoPosixSemantics covers Root.Rename onto a name surrogate on a
// kernel without FileRenameInformationEx. The fallback is forced on so that it
// is covered on every Windows version. A junction stands in for a directory
// symlink, since creating one needs no privilege.
func TestRenameatNoPosixSemantics(t *testing.T) {
	output, _ := testenv.Command(t, "cmd", "/c", "mklink", "/?").Output()
	if !strings.Contains(string(output), " /J ") {
		t.Skip("skipping test because mklink command does not support junctions")
	}
	windows.TestRenameatNoPosixSemantics = true
	t.Cleanup(func() { windows.TestRenameatNoPosixSemantics = false })

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if out, err := testenv.Command(t, "cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J %v %v: %v\n%s", link, target, err, out)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.Rename("source", "link"); err != nil {
		t.Fatal(err)
	}

	// The junction is gone, replaced by the file, and what it pointed at is
	// untouched.
	if b, err := os.ReadFile(link); err != nil || string(b) != "source" {
		t.Errorf("after rename, ReadFile(link) = %q, %v, want %q, <nil>", b, err, "source")
	}
	if fi, err := os.Lstat(link); err != nil {
		t.Error(err)
	} else if fi.Mode()&fs.ModeSymlink != 0 {
		t.Errorf("after rename, Lstat(link).Mode() = %v, want a regular file", fi.Mode())
	}
	if fi, err := os.Stat(target); err != nil {
		t.Error(err)
	} else if !fi.IsDir() {
		t.Errorf("after rename, Stat(target).IsDir() = false, want true")
	}
	if _, err := os.Lstat(filepath.Join(dir, "source")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("after rename, Lstat(source) = %v, want ErrNotExist", err)
	}
}

// TestRenameatReplaceLeavesNoStray covers a rename onto a file another handle
// holds open, through the fallback, which moves the destination aside before
// renaming. Once the rename has landed, the moved copy has to be gone, so that
// nothing is left behind in the directory. Only the fallback is covered, as in
// TestRenameatNoPosixSemantics.
func TestRenameatReplaceLeavesNoStray(t *testing.T) {
	windows.TestRenameatNoPosixSemantics = true
	t.Cleanup(func() { windows.TestRenameatNoPosixSemantics = false })

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "target"), []byte("target"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	// Hold both open across the rename, the way a listing that forgot to
	// close its files would. Root.Open shares deletion, so the rename is
	// allowed to try. It is the destination's name that stands in the way,
	// which the older rename refuses to replace, so the fallback has to
	// move it aside.
	for _, name := range []string{"source", "target"} {
		f, err := root.Open(name)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
	}
	if err := root.Rename("source", "target"); err != nil {
		t.Fatal(err)
	}

	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range names {
		got = append(got, e.Name())
	}
	if len(got) != 1 || got[0] != "target" {
		t.Errorf("directory holds %v after the rename, want only target", got)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "target")); err != nil || string(b) != "source" {
		t.Errorf("target holds %q, %v; want %q", b, err, "source")
	}
}

// TestRenameatRetryFailsRestoresTarget covers the fallback when the rename
// retried after the destination was moved aside fails. The destination has to
// come back under its name with its content, and the source has to stay.
func TestRenameatRetryFailsRestoresTarget(t *testing.T) {
	windows.TestRenameatNoPosixSemantics = true
	t.Cleanup(func() { windows.TestRenameatNoPosixSemantics = false })
	windows.TestRenameatRetryFails = true
	t.Cleanup(func() { windows.TestRenameatRetryFails = false })

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "target"), []byte("target"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	held, err := root.Open("target")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	if err := root.Rename("source", "target"); err == nil {
		t.Fatal("Rename succeeded with the retry forced to fail")
	}
	for name, want := range map[string]string{"source": "source", "target": "target"} {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(b) != want {
			t.Errorf("%s holds %q, %v; want %q", name, b, err, want)
		}
	}
	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Errorf("directory holds %d entries after the refused rename, want 2", len(names))
	}
}

// TestRenameatDirOntoHeldFileKeepsFile covers renaming a directory onto a
// file another handle holds open, through the fallback. A rename with POSIX
// semantics refuses that, so the fallback has to as well, and the file has to
// survive it. The fallback is forced on as in TestRenameatNoPosixSemantics.
func TestRenameatDirOntoHeldFileKeepsFile(t *testing.T) {
	windows.TestRenameatNoPosixSemantics = true
	t.Cleanup(func() { windows.TestRenameatNoPosixSemantics = false })

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "source"), 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "target"), []byte("target"), 0600); err != nil {
		t.Fatal(err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	held, err := root.Open("target")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	if err := root.Rename("source", "target"); err == nil {
		t.Fatal("Rename of a directory onto a held file succeeded")
	}
	if b, err := os.ReadFile(filepath.Join(dir, "target")); err != nil || string(b) != "target" {
		t.Errorf("target holds %q, %v; want %q", b, err, "target")
	}
}

// TestRenameatReadOnlyTargetKept covers renaming onto a read-only file
// through the fallback. A rename with POSIX semantics refuses that, so the
// fallback has to as well, and the file has to survive it. The fallback is
// forced on as in TestRenameatNoPosixSemantics.
func TestRenameatReadOnlyTargetKept(t *testing.T) {
	windows.TestRenameatNoPosixSemantics = true
	t.Cleanup(func() { windows.TestRenameatNoPosixSemantics = false })

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "source"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "target"), []byte("target"), 0400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Join(dir, "target"), 0600) })

	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.Rename("source", "target"); err == nil {
		t.Fatal("Rename onto a read-only file succeeded")
	}
	for name, want := range map[string]string{"source": "source", "target": "target"} {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(b) != want {
			t.Errorf("%s holds %q, %v; want %q", name, b, err, want)
		}
	}
}

// TestRemoveFallbackReadOnlySwapped checks that the fallback clears the
// read-only attribute only on the entry it is deleting. Between the open that
// found the attribute and the open that clears it, the name is given to
// another read-only directory, which has to keep its attribute. Whether the
// original can still be deleted with its attribute in place is up to the
// kernel and is not checked. Only the fallback is covered, for the reason
// given at TestRemoveFallbackOpenFile.
func TestRemoveFallbackReadOnlySwapped(t *testing.T) {
	windows.TestDeleteatFallback = true
	t.Cleanup(func() { windows.TestDeleteatFallback = false })

	dir := t.TempDir()
	for _, name := range []string{"target", "other"} {
		// Mkdir does not set the read-only attribute on Windows, Chmod does.
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(filepath.Join(dir, name), 0500); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, name := range []string{"target", "other", "moved"} {
			os.Chmod(filepath.Join(dir, name), 0700)
		}
	})
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	windows.TestDeleteatBeforeReopen = func() {
		windows.TestDeleteatBeforeReopen = nil
		if err := root.Rename("target", "moved"); err != nil {
			t.Error(err)
		}
		if err := root.Rename("other", "target"); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(func() { windows.TestDeleteatBeforeReopen = nil })

	root.Remove("target")
	fi, err := os.Stat(filepath.Join(dir, "target"))
	if err != nil {
		t.Fatalf("the directory that took the name: %v", err)
	}
	if fi.Mode()&0200 != 0 {
		t.Error("the directory that took the name lost its read-only attribute")
	}
}
