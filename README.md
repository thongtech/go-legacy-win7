# The Go Programming Language

**go-legacy-win7** is a fork of the Go programming language that maintains support for Windows 7, 8, 8.1, Server 2008 R2, Server 2012, and Server 2012 R2, and restores the deprecated `go get` behaviour. This project aims to provide a stable Go environment for users who need to support legacy Windows systems or prefer the traditional Go workflow.

![Gopher image](https://golang.org/doc/gopher/fiveyears.jpg)
_Gopher image by [Renee French][rf], licensed under [Creative Commons 4.0 Attribution licence][cc4-by]._

## Differences from Upstream Go

1. **Windows 7/8/8.1 and Legacy Server Support**  
   While the official Go project dropped support for Windows 7, 8, 8.1, Server 2008 R2, Server 2012, and Server 2012 R2 in Go 1.21, this fork maintains compatibility with all these legacy Windows systems.

   Tested on Windows 7 RTM (build 7600) — no updates required — through Windows 11 25H2

2. **Classic `go get` Behaviour**  
   This fork allows for the deprecated `go get` behaviour when `GO111MODULE` is set to "off" or "auto". This means:

   - In `GOPATH/src`, `go get` and `go install` can operate in `GOPATH` mode.
   - Outside of `GOPATH/src`, these commands can use module-aware mode when appropriate.

3. **Compatibility Notes**  
   Please be aware that some newer Go features may not be fully compatible with legacy Windows systems. We try to maintain as much functionality as possible, but some limitations may exist.

## Changes in Each Release

Current release includes the following modifications:

- Switched back to `RtlGenRandom` from `ProcessPrng`, which breaks Win7/2008R2 (reverted [693def1](https://github.com/golang/go/commit/693def151adff1af707d82d28f55dba81ceb08e1))
- Added back `LoadLibraryA` fallback to load system libraries (reverted [a17d959](https://github.com/golang/go/commit/a17d959debdb04cd550016a3501dd09d50cd62e7))
- Added back `sysSocket` fallback for socket syscalls (reverted [7c1157f](https://github.com/golang/go/commit/7c1157f9544922e96945196b47b95664b1e39108))
- Added back Windows 7 console handle workaround (reverted [48042aa](https://github.com/golang/go/commit/48042aa09c2f878c4faa576948b07fe625c4707a))
- Added back 5ms sleep on Windows 7/8 in (\*Process).Wait (reverted [f0894a0](https://github.com/golang/go/commit/f0894a00f4b756d4b9b4078af2e686b359493583))
- Restored deprecated `go get` behaviour for use outside modules (reverted [de4d503](https://github.com/golang/go/commit/de4d50316fb5c6d1529aa5377dc93b26021ee843))
- Reverted to the previous `removeall_noat` variant for Windows (fixed [issue #2](https://github.com/thongtech/go-legacy-win7/issues/2))
- Rolled back `race_windows.syso` to the previous compatible version (fixed [issue #3](https://github.com/thongtech/go-legacy-win7/issues/3))
- Added `FindFirstFile`/`FindNextFile` fallback for old SMB shares (fixed [issue #9](https://github.com/thongtech/go-legacy-win7/issues/9))
- Includes all improvements and bug fixes from the corresponding upstream Go release

We now provide two build options for Windows amd64:

- **Standard build**  
  Maximum compatibility with Windows 7, 8, 8.1, Server 2008 R2, Server 2012, and Server 2012 R2 with the reverted race detector. Use this if you need legacy Windows support.

- **Race detector build** (version suffix `-race`)  
  Uses Go's latest stable race detector without modifications. Recommended for Windows 10+ when running race tests to avoid potential edge cases with the legacy race detector. See [issue #6](https://github.com/thongtech/go-legacy-win7/issues/6)

## Download and Install

### Binary Distributions

Download builds from GitHub Releases:

- **[Latest release](https://github.com/thongtech/go-legacy-win7/releases/latest)** — current recommended version
- **[All releases](https://github.com/thongtech/go-legacy-win7/releases)** — every published version, including older Go lines

### Before you begin
To avoid PATH/GOROOT conflicts and mixed toolchains, uninstall any existing Go installation first.

#### Windows Installation

1. Download the `go-legacy-win7-<version>.windows_<arch>.zip` file.
2. Extract the ZIP to `C:\` (or any preferred location). This will create a `go-legacy-win7` folder.
3. Add the following to your system environment variables:
   - Add `C:\go-legacy-win7\bin` (or your chosen path) to the system `PATH`.
   - Set `GOROOT` to `C:\go-legacy-win7` (or your chosen path).
4. Add the following to your user environment variables:
   - Add `%USERPROFILE%\go\bin` to the user `PATH`.
   - Set `GOPATH` to `%USERPROFILE%\go`.

#### macOS and Linux Installation

1. Download the appropriate `go-legacy-win7-<version>.<os>_<arch>.tar.gz` file.

   - For macOS: `go-legacy-win7-<version>.darwin_<arch>.tar.gz`
   - For Linux: `go-legacy-win7-<version>.linux_<arch>.tar.gz`

2. Extract the archive to `/usr/local`:

   ```
   sudo tar -C /usr/local -xzf go-legacy-win7-<version>.<os>_<arch>.tar.gz
   ```

3. Add the following to your shell configuration file:

   - For bash, add to `~/.bash_profile` or `~/.bashrc`
   - For zsh, add to `~/.zshrc`

   ```bash
   export GOROOT=/usr/local/go-legacy-win7
   export GOPATH=$HOME/go
   export PATH=$PATH:$GOROOT/bin:$GOPATH/bin
   ```

4. Apply the changes:

   - For bash: `source ~/.bash_profile` or `source ~/.bashrc`
   - For zsh: `source ~/.zshrc`

   Note:

   - On macOS Catalina and later, zsh is the default shell.
   - On most Linux distributions, bash is the default shell.

After installation, verify the installation by opening a **new terminal** and running:

```
go version
```

### Install From Source

To install from source, please follow the steps on the [official website](https://go.dev/doc/install/source).

## FAQ

**Is this official Go?**  
No. It is an independent fork. Bug reports for this fork belong here. Language and spec issues still belong with upstream Go.

**Why not just use an old Go release?**  
Old releases miss years of fixes and language/runtime updates. This fork tracks current Go releases while retaining the legacy Windows and `go get` support.

**Will you drop Windows 7 or raise system requirements?**  
No. Supporting Windows 7 and later is a standing promise. We aim to keep things working. We do not “fix” what is not broken.

**What about security on legacy Windows?**  
Older Windows already has a large attack surface. Hardening individual legacy APIs here would not make those systems meaningfully safer. We prioritise stable behaviour instead. Prefer a Windows version Microsoft still supports when you can. This fork exists for cases where you cannot.

**Is this safe to use on modern Windows?**  
Usually yes. Legacy APIs we keep still work on current Windows, but that is not guaranteed forever. Some patches reimplement behaviour that newer Windows provides natively, so there can be a small performance cost. If you want the full modern Windows/API path, build with the [official Go toolchain](https://go.dev/dl/) and use this fork only for legacy targets.

**Should I use the standard build or the `-race` build?**  
Use the **standard** build for maximum legacy Windows compatibility (including the reverted race detector). Use the **`-race`** Windows amd64 build on Windows 10+ when you need race testing with upstream’s newer race detector.

**Can I mix this with an official Go install?**  
Avoid it. Uninstall other Go installs first (or keep isolated `GOROOT`/`PATH`) so tools do not pick the wrong binary.

## Contributing

Feedback and issue reports are welcome, and we encourage you to open pull requests to contribute to the project. We appreciate your help!

Note that the Go project uses the issue tracker for bug reports and
proposals only. See https://go.dev/wiki/Questions for a list of
places to ask questions about the Go language.

[rf]: https://reneefrench.blogspot.com/
[cc4-by]: https://creativecommons.org/licenses/by/4.0/
