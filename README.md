# The Go Programming Language

**go-legacy-win7** is a fork of the Go programming language that maintains support for Windows 7, 8, 8.1, Server 2008 R2, Server 2012, and Server 2012 R2, and restores the deprecated `go get` behaviour. This project aims to provide a stable Go environment for users who need to support legacy Windows systems or prefer the traditional Go workflow.

![Gopher image](https://golang.org/doc/gopher/fiveyears.jpg)
_Gopher image by [Renee French][rf], licensed under [Creative Commons 4.0 Attribution licence][cc4-by]._

## Differences from Upstream Go

1. **Windows 7/8/8.1 and Legacy Server Support**  
   While the official Go project dropped support for Windows 7, 8, 8.1, Server 2008 R2, Server 2012, and Server 2012 R2 in Go 1.21, this fork maintains compatibility with all these legacy Windows systems.

   Tested on Windows 7 RTM (build 7600) — no updates required — through Windows 11 25H2

2. **Upstream Code First, Not a Blind Revert**  
   Since `1.26.8-1` and `1.27.1-1`, legacy support is no longer restored by reverting upstream changes or freezing old code. Upstream's current implementation is kept, with a fallback beneath it chosen at runtime by what the machine actually supports. Each fallback is built to reproduce upstream's behaviour as closely as the platform allows, so a program behaves the same whether it runs on Windows 7 or Windows 11, and current Windows is unaffected.

3. **Classic `go get` Behaviour**  
   This fork allows for the deprecated `go get` behaviour when `GO111MODULE` is set to "off" or "auto". This means:

   - In `GOPATH/src`, `go get` and `go install` can operate in `GOPATH` mode.
   - Outside of `GOPATH/src`, these commands can use module-aware mode when appropriate.

4. **Compatibility Notes**  
   Please be aware that some newer Go features may not be fully compatible with legacy Windows systems. We try to maintain as much functionality as possible, but some limitations may exist. If you find one, please [report it](https://github.com/thongtech/go-legacy-win7/issues).

## Changes in Each Release

Current release includes the following modifications:

| Change | Patches | Broken by |
| --- | --- | --- |
| The runtime and `syscall` load system libraries by absolute path where the restricted search flag is not understood. | [0001](patches/0001-runtime-syscall-fall-back-to-loading-system-dlls-by-absolute-path.patch) | [a17d959](https://github.com/golang/go/commit/a17d959debdb04cd550016a3501dd09d50cd62e7) (Go 1.21) |
| The runtime and `crypto/rand` fall back to `RtlGenRandom` where `ProcessPrng` is absent, which is every Windows before 8. | [0002](patches/0002-runtime-crypto-internal-sysrand-fall-back-to-rtlgenrandom-when-processprng-is-absent.patch) | [693def1](https://github.com/golang/go/commit/693def151adff1af707d82d28f55dba81ceb08e1) (Go 1.22, and 1.21.5 by backport) |
| `syscall` duplicates console handles in `StartProcess` the way Windows 7 requires. | [0003](patches/0003-syscall-restore-windows-7-console-handle-handling-in-startprocess.patch) | [2d76081](https://github.com/golang/go/commit/2d760816ff30bea82f54682f3049cfb6c6027da7) (Go 1.17)<br>[48042aa](https://github.com/golang/go/commit/48042aa09c2f878c4faa576948b07fe625c4707a) (Go 1.22) |
| `os` reads directories through `FILE_ID_BOTH_DIR_INFO` where the newer information classes are unavailable. | [0004](patches/0004-os-fall-back-to-file-id-both-dir-info-when-reading-directories.patch) | [be0b8e8](https://github.com/golang/go/commit/be0b8e84b09733ddc6f36eca489193fe974accc9) (Go 1.22)<br>[2860e01](https://github.com/golang/go/commit/2860e01853174e278900ef6907b1941b16fb1645) (Go 1.23) |
| `os` opens the console devices under the names Windows 7 reports as missing. | [0005](patches/0005-os-open-the-console-devices-by-the-names-windows-7-accepts.patch) | [28b8851](https://github.com/golang/go/commit/28b8851671a0254ed0e46ce8dbec43ebe73e7132) (Go 1.23)<br>[bb0c14b](https://github.com/golang/go/commit/bb0c14b895d90bb5941e0463ba6c3564fc504e4f) (Go 1.25) |
| `net` creates a socket without `WSA_FLAG_NO_HANDLE_INHERIT` where that flag is rejected, and clears the inherit flag afterwards. | [0006](patches/0006-net-fall-back-when-wsa-flag-no-handle-inherit-is-rejected.patch) | [7c1157f](https://github.com/golang/go/commit/7c1157f9544922e96945196b47b95664b1e39108) (Go 1.22) |
| `internal/poll` keeps files off a completion port they could not leave, with deadlines of their own, and drives a socket that cannot leave one with an event, so `os.File.Fd` hands back a detached handle wherever the kernel allows it and connections still work where it does not. | [0007](patches/0007-internal-poll-keep-a-handle-that-cannot-leave-its-completion-port.patch)<br>[0009](patches/0009-internal-poll-keep-file-handles-off-a-completion-port-they-cannot-leave.patch) | [8a8f506](https://github.com/golang/go/commit/8a8f506516e1210c9ca3a352d76bd1d570c407fd) (Go 1.25)<br>[6953ef8](https://github.com/golang/go/commit/6953ef86cd72a835d398319c4da560c8b78ba28e) (Go 1.25) |
| `internal/poll` no longer hangs on a datagram receive that truncates. | [0008](patches/0008-internal-poll-do-not-skip-the-completion-port-for-datagram-sockets.patch) | [932d0ae](https://github.com/golang/go/commit/932d0ae83ea8ce9adb2b23b28788b860447b1f61) (Go 1.21) |
| `internal/syscall/windows` and `os` open, rename and delete files and directories through fallbacks where the kernel rejects the newer calls. That covers read-only directories that `os.RemoveAll` left behind, and a file that is deleted, through `os.Remove` as well, or replaced by a rename gives its name up at once rather than holding it until the last handle closes. | [0010](patches/0010-internal-syscall-windows-work-around-a-rejected-obj-dont-reparse.patch)<br>[0011](patches/0011-internal-syscall-windows-clear-the-read-only-attribute-on-a-directory.patch)<br>[0012](patches/0012-internal-syscall-windows-os-free-the-name-before-a-delete-completes.patch)<br>[0013](patches/0013-internal-syscall-windows-replace-a-rename-target-where-posix-rename-is-missing.patch) | [86a1a99](https://github.com/golang/go/commit/86a1a994ff522a7236e6744e40dfbc33d0d6bd88) (Go 1.24)<br>[2ffda87](https://github.com/golang/go/commit/2ffda87f2dce71024f72ccff32cbfe29ee676bf8) (Go 1.25)<br>[b31dc77](https://github.com/golang/go/commit/b31dc77ceab962c0f4f5e4a9fc5e1a403fbd2d7c) (Go 1.26)<br>[6d41809](https://github.com/golang/go/commit/6d418096b2dfe2a2e47b7aa83b46748fb301e6cb) (Go 1.25) |
| `os.Root.Symlink` passes `DeviceIoControl` a place to store the byte count. Windows 7 writes it regardless, and without one the process died at address 0. Upstream fixed this in Go 1.27 and did not backport it. | [0014](patches/0014-internal-syscall-windows-pass-lpbytesreturned-when-setting-a-reparse-point.patch) | [26fdb07](https://github.com/golang/go/commit/26fdb07d4ce58885305283ba18960f582f4eaa73) (Go 1.25) |
| `-race` binaries no longer fail to load for `api-ms-win-core-synch-l1-2-0.dll`. | [0015](patches/0015-runtime-race-cmd-link-let-race-binaries-start-on-windows-7.patch) | [c3bea70](https://github.com/golang/go/commit/c3bea70d9b3b80ceb7733cd7bde0cdf0a1bfd0d0) (Go 1.19)<br>[cc82867](https://github.com/golang/go/commit/cc82867f6bf650e6b48a6e87849e4fdd5b94ef70) (Go 1.21) |
| The `go` command runs GOPATH-mode `go get` outside a module again. | [9999](patches/9999-cmd-go-restore-gopath-mode-go-get.patch) | [de4d503](https://github.com/golang/go/commit/de4d50316fb5c6d1529aa5377dc93b26021ee843) (Go 1.22) |
| Three test-only changes, each skipping a check where the machine cannot supply what it needs. The `T` sequence touches test files only and changes nothing a program does. | [T0001](patches/T0001-path-filepath-skip-the-vhd-tests-where-the-hyper-v-cmdlets-are-missing.patch)<br>[T0002](patches/T0002-crypto-tls-skip-the-live-resumption-check-when-the-chain-cannot-be-verified.patch)<br>[T0003](patches/T0003-crypto-x509-skip-system-verify-cases-whose-root-the-store-lacks.patch) | [7986e26](https://github.com/golang/go/commit/7986e26a39e9df870886a9933107372f4e16ea4c) (Go 1.23)<br>[b691a2e](https://github.com/golang/go/commit/b691a2edc7f5863f61a07c4a4f087eef1a15a704) (Go 1.27)<br>[bb8f9a6](https://github.com/golang/go/commit/bb8f9a6ae66d742cb67b4ad444179905a537de00) (Go 1.21) |

Every fallback is coupled with tests, and the fork passes Go's full test suite on Windows 7 x64.

## Download and Install

### Binary Distributions

Download builds from GitHub Releases:

- **[Latest release](https://github.com/thongtech/go-legacy-win7/releases/latest)** — current recommended version
- **[All releases](https://github.com/thongtech/go-legacy-win7/releases)** — every published version, including older Go lines

### Before you begin
To avoid PATH/GOROOT conflicts and mixed toolchains, uninstall any existing Go installation first.

<details open>
<summary><strong>Windows Installation</strong></summary>

1. Download the `go-legacy-win7-<version>.windows_<arch>.zip` file.
2. Extract the ZIP to `C:\` (or any preferred location). This will create a `go-legacy-win7` folder.
3. Add the following to your system environment variables:
   - Add `C:\go-legacy-win7\bin` (or your chosen path) to the system `PATH`.
   - Set `GOROOT` to `C:\go-legacy-win7` (or your chosen path).
4. Add the following to your user environment variables:
   - Add `%USERPROFILE%\go\bin` to the user `PATH`.
   - Set `GOPATH` to `%USERPROFILE%\go`.

</details>

<details>
<summary><strong>macOS and Linux Installation</strong></summary>

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

</details>

<br>

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
Older Windows already has a large attack surface. Hardening individual legacy APIs here would not make those systems meaningfully safer. We prioritise stable behaviour instead, without weakening what upstream Go already guarantees. Prefer a Windows version Microsoft still supports when you can. This fork exists for cases where you cannot.

**Is this safe to use on modern Windows?**  
Yes. The fork always prefers upstream's implementation and falls back only where the running system cannot provide it, so on current Windows you get exactly the APIs upstream Go uses. Neither side pays for the other. Legacy systems gain support they would otherwise not have, and modern systems give up nothing measurable in return.

**Do I need a separate `-race` build?**  
No, not anymore. Earlier releases split the build because `-race` binaries could not start on Windows 7. That gap is closed, so one build now covers every supported Windows version.

**Can I mix this with an official Go install?**  
Avoid it. Uninstall other Go installs first (or keep isolated `GOROOT`/`PATH`) so tools do not pick the wrong binary.

## Contributing

Feedback and issue reports are welcome, and we encourage you to open pull requests to contribute to the project. We appreciate your help!

Note that the Go project uses the issue tracker for bug reports and
proposals only. See https://go.dev/wiki/Questions for a list of
places to ask questions about the Go language.

[rf]: https://reneefrench.blogspot.com/
[cc4-by]: https://creativecommons.org/licenses/by/4.0/
