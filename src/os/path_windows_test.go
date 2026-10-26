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

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
}

// TestRemoveOpenFile checks that os.Remove of a file still open frees its
// name at once, as DeleteFileW does where the kernel has POSIX delete.
func TestRemoveOpenFile(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "file")
	if err := os.WriteFile(name, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	// Root.Open shares deletion, so the file can be removed while held.
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

	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(name); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Lstat after Remove = %v, want ErrNotExist", err)
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
}

// TestRemoveFallbackAnswers checks that the fallback's errors for a missing
// name, a missing path and a directory are DeleteFileW's and
// RemoveDirectoryW's.
func TestRemoveFallbackAnswers(t *testing.T) {
	dir := t.TempDir()
	full := filepath.Join(dir, "full")
	if err := os.MkdirAll(filepath.Join(full, "entry"), 0777); err != nil {
		t.Fatal(err)
	}
	mkdir := func(name string) func() {
		return func() {
			if err := os.Mkdir(filepath.Join(dir, name), 0777); err != nil {
				t.Fatal(err)
			}
		}
	}
	write := func(name string) func() {
		return func() {
			if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0666); err != nil {
				t.Fatal(err)
			}
		}
	}
	cases := []struct {
		name string
		path string
		make func() // makes what the remove is to find, before each remove
		ok   bool   // the remove must succeed
	}{
		{"missing name", filepath.Join(dir, "missing"), nil, false},
		{"missing path", filepath.Join(dir, "missing", "name"), nil, false},
		{"empty directory", filepath.Join(dir, "empty"), mkdir("empty"), true},
		{"directory with an entry", full, nil, false},
		// The name resolves as DeleteFileW and RemoveDirectoryW resolve it,
		// without regard to case.
		{"differently cased file", filepath.Join(dir, "CASED.TXT"), write("cased.txt"), true},
		{"differently cased directory", filepath.Join(dir, "CASEDIR"), mkdir("casedir"), true},
	}
	errs := make([][]error, 2)
	for i, hook := range []bool{false, true} {
		windows.TestDeleteatFallback = hook
		t.Cleanup(func() { windows.TestDeleteatFallback = false })
		for _, c := range cases {
			if c.make != nil {
				c.make()
			}
			err := os.Remove(c.path)
			if c.ok && err != nil {
				t.Errorf("hook %v: Remove of %s: %v", hook, c.name, err)
			}
			errs[i] = append(errs[i], err)
		}
	}
	for j, c := range cases {
		a, b := errs[0][j], errs[1][j]
		if (a == nil) != (b == nil) || (a != nil && a.(*os.PathError).Err != b.(*os.PathError).Err) {
			t.Errorf("Remove of %s: %v without the hook, %v with it", c.name, a, b)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "empty")); !os.IsNotExist(err) {
		t.Errorf("empty directory still there after Remove: %v", err)
	}
}

// TestRemoveFallbackReadOnlyAlone checks that a file whose only attribute is
// read-only is deleted, which needs the normal attribute written.
func TestRemoveFallbackReadOnlyAlone(t *testing.T) {
	windows.TestDeleteatFallback = true
	t.Cleanup(func() { windows.TestDeleteatFallback = false })
	name := filepath.Join(t.TempDir(), "ro")
	if err := os.WriteFile(name, []byte("x"), 0666); err != nil {
		t.Fatal(err)
	}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_READONLY); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(name); err != nil {
		syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_NORMAL)
		t.Fatalf("Remove of a file with the read-only attribute alone: %v", err)
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Errorf("file still there after Remove: %v", err)
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

// TestRenameatReplaceLeavesNoStray checks that a rename onto a held file
// through the fallback leaves no moved copy behind.
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

	// Root.Open shares deletion, so the rename may try, and the held
	// target has to be moved aside.
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

// TestRenameatRetryFailsRestoresTarget checks that a failed retry puts the
// target back with its content and leaves the source.
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

// TestRenameatReadOnlyTargetKept checks that the fallback refuses a rename
// onto a read-only file, as POSIX rename does.
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

// TestRemoveFallbackReadOnlySwapped checks that the read-only attribute is
// cleared only on the entry being deleted, not on one given its name in
// between.
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
