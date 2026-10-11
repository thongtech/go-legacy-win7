# The Go Programming Language

**go-legacy-win7** is a fork of the Go programming language that maintains support for Windows 7, 8, 8.1, Server 2008 R2, Server 2012, and Server 2012 R2, and restores the deprecated `go get` behaviour. This project aims to provide a stable Go environment for users who need to support legacy Windows systems or prefer the traditional Go workflow.

![Gopher image](https://golang.org/doc/gopher/fiveyears.jpg)
_Gopher image by [Renee French][rf], licensed under [Creative Commons 4.0 Attribution licence][cc4-by]._

## Differences from Upstream Go

1. **Windows 7/8/8.1 and Legacy Server Support**  
   While the official Go project dropped support for Windows 7, 8, 8.1, Server 2008 R2, Server 2012, and Server 2012 R2 in Go 1.21, this fork maintains compatibility with all these legacy Windows systems.

   Tested on Windows 7 (build 7600) — no updates required — through Windows 11 26H2 (build 26300)

2. **Upstream Code First, Not a Blind Revert**  
   Legacy support is not restored by reverting upstream changes or freezing old code. Upstream's current implementation is kept, with a fallback beneath it chosen at runtime by what the machine actually supports. Each fallback is built to reproduce upstream's behaviour as closely as the platform allows, so a program behaves the same whether it runs on Windows 7 or Windows 11, and current Windows is unaffected.

3. **Classic `go get` Behaviour**  
   This fork allows for the deprecated `go get` behaviour when `GO111MODULE` is set to "off" or "auto". This means:

   - In `GOPATH/src`, `go get` and `go install` can operate in `GOPATH` mode.
   - Outside of `GOPATH/src`, these commands can use module-aware mode when appropriate.

4. **Compatibility Notes**  
   Please be aware that some newer Go features may not be fully compatible with legacy Windows systems. We try to maintain as much functionality as possible, but some limitations may exist. If you find one, please [report it](https://github.com/thongtech/go-legacy-win7/issues).

## Changes in Each Release

Current release includes the following modifications. The last column names
the upstream change each patch works around and the Go release it shipped in,
or the kernel where there is none. The `T` patches touch tests only.

### Loader and startup

| What it fixes | Patch | Upstream change |
| --- | --- | --- |
| Every program dies at startup on Windows 7 without KB2533623. The runtime and `syscall` load system DLLs with a search flag that update introduced. Where the flag is rejected they load the file by its full path in the system directory. | [0002](patches/0002-runtime-syscall-fall-back-to-loading-system-dlls-by-absolute-path.patch) | [a17d959](https://github.com/golang/go/commit/a17d959debdb04cd550016a3501dd09d50cd62e7) (Go 1.21) |
| Every program dies at startup on Windows 7, which has no `ProcessPrng`. The runtime and `crypto/rand` fall back to `RtlGenRandom` where it is missing. | [0003](patches/0003-runtime-crypto-internal-sysrand-fall-back-to-rtlgenrandom-when-processprng-is-absent.patch) | [693def1](https://github.com/golang/go/commit/693def151adff1af707d82d28f55dba81ceb08e1) (Go 1.22, and 1.21.5 by backport) |

### Processes and consoles

| What it fixes | Patch | Upstream change |
| --- | --- | --- |
| Starting a child process from a console fails on Windows 7, the `go` command included. Windows 7 console handles are not real handles. `StartProcess` treats them as Go 1.21 did. | [0004](patches/0004-syscall-restore-windows-7-console-handle-handling-in-startprocess.patch) | [2d76081](https://github.com/golang/go/commit/2d760816ff30bea82f54682f3049cfb6c6027da7) (Go 1.17)<br>[48042aa](https://github.com/golang/go/commit/48042aa09c2f878c4faa576948b07fe625c4707a) (Go 1.22) |
| Opening or stating `CON`, `CONIN$` or `CONOUT$` fails on Windows 7. The `\\.\` names reach the console only from Windows 8. `os` retries with the bare names. | [0006](patches/0006-os-open-the-console-devices-by-the-names-windows-7-accepts.patch) | [28b8851](https://github.com/golang/go/commit/28b8851671a0254ed0e46ce8dbec43ebe73e7132) (Go 1.23)<br>[bb0c14b](https://github.com/golang/go/commit/bb0c14b895d90bb5941e0463ba6c3564fc504e4f) (Go 1.25) |

### Network and the poller

| What it fixes | Patch | Upstream change |
| --- | --- | --- |
| Every socket creation fails on Windows 7 (build 7600). `WSA_FLAG_NO_HANDLE_INHERIT` needs Service Pack 1. `net` retries without it and clears inheritance afterwards. | [0007](patches/0007-net-fall-back-when-wsa-flag-no-handle-inherit-is-rejected.patch) | [7c1157f](https://github.com/golang/go/commit/7c1157f9544922e96945196b47b95664b1e39108) (Go 1.22) |
| `File` on a connection or listener fails before Windows 8.1. Nothing before 8.1 can take a handle off a completion port. The handle is left where it is and still works, and `Close` still ends a pending read on it. | [0008](patches/0008-internal-poll-keep-a-handle-that-cannot-leave-its-completion-port.patch) | [8a8f506](https://github.com/golang/go/commit/8a8f506516e1210c9ca3a352d76bd1d570c407fd) (Go 1.25)<br>[6953ef8](https://github.com/golang/go/commit/6953ef86cd72a835d398319c4da560c8b78ba28e) (Go 1.25) |
| A UDP read into a buffer too small for a datagram already queued hangs forever on Windows 7. Datagram sockets no longer skip the completion packet there. | [0009](patches/0009-internal-poll-do-not-skip-the-completion-port-for-datagram-sockets.patch) | [932d0ae](https://github.com/golang/go/commit/932d0ae83ea8ce9adb2b23b28788b860447b1f61) (Go 1.21) |
| `os.File.Fd` cannot hand back a detached handle before Windows 8.1. Files and pipes are kept off the runtime's completion port there, with deadlines of their own, so `Fd` returns a detached handle. | [0010](patches/0010-internal-poll-keep-file-handles-off-a-completion-port-they-cannot-leave.patch) | [8a8f506](https://github.com/golang/go/commit/8a8f506516e1210c9ca3a352d76bd1d570c407fd) (Go 1.25)<br>[d889729](https://github.com/golang/go/commit/d889729733d82de417567926e53bd816211852b4) (Go 1.28, and 1.26.9 and 1.27.2 by backport) |
| Every listener in the process stops accepting on Windows 8 and Server 2012 while a thread reads a pipe. The kernel holds an `AcceptEx` completion until the posting thread leaves synchronous I/O. `AcceptEx` is posted from a thread that never does. | [0018](patches/0018-internal-poll-post-acceptex-from-a-thread-that-never-blocks-in-synchronous-io.patch) | [Connect #760161](https://serverframework.com/asynchronousevents/2012/09/acceptex-bug-in-windows-8-and-all-server-2012-variants.html) (Windows 8)<br>Kernel |

### Files

| What it fixes | Patch | Upstream change |
| --- | --- | --- |
| Listing a directory fails on Windows 7 on FAT32, exFAT and network shares. The newer directory information class arrived in Windows 8. `os` reads with the older one where the kernel rejects the new. | [0005](patches/0005-os-fall-back-to-file-id-both-dir-info-when-reading-directories.patch) | [be0b8e8](https://github.com/golang/go/commit/be0b8e84b09733ddc6f36eca489193fe974accc9) (Go 1.22)<br>[2860e01](https://github.com/golang/go/commit/2860e01853174e278900ef6907b1941b16fb1645) (Go 1.23) |
| `os.RemoveAll` leaves a read-only directory behind before Windows 10 1809. The read-only attribute is cleared on a directory as it is on a file. | [0011](patches/0011-internal-syscall-windows-clear-the-read-only-attribute-on-a-directory.patch) | [b31dc77](https://github.com/golang/go/commit/b31dc77ceab962c0f4f5e4a9fc5e1a403fbd2d7c) (Go 1.26)<br>[6d41809](https://github.com/golang/go/commit/6d418096b2dfe2a2e47b7aa83b46748fb301e6cb) (Go 1.25) |
| A deleted file keeps its name while another handle holds it, where the kernel has no POSIX delete. A held file is moved aside before it is marked, so the name is free at once, through `os.Remove` too. | [0012](patches/0012-internal-syscall-windows-os-free-the-name-before-a-delete-completes.patch) | [b31dc77](https://github.com/golang/go/commit/b31dc77ceab962c0f4f5e4a9fc5e1a403fbd2d7c) (Go 1.26) |
| Deleting a program right after it exits fails on Windows 8 and 8.1. The kernel keeps the image mapped for a moment after `Wait` returns. The delete waits that out. | [0013](patches/0013-internal-syscall-windows-wait-out-a-just-run-image-where-the-kernel-keeps-it-mapped.patch) | [f0894a0](https://github.com/golang/go/commit/f0894a00f4b756d4b9b4078af2e686b359493583) (Go 1.22) |
| `os.RemoveAll` finds a directory not empty while an entry already marked for deletion is on its way out. The delete waits for the name to go. | [0014](patches/0014-internal-syscall-windows-wait-for-a-deleted-name-its-last-handle-is-about-to-release.patch) | [b31dc77](https://github.com/golang/go/commit/b31dc77ceab962c0f4f5e4a9fc5e1a403fbd2d7c) (Go 1.26)<br>Kernel |
| Renaming onto a link or a held file fails where the kernel has no POSIX rename. The target is moved aside, the rename retried, and the copy deleted. | [0015](patches/0015-internal-syscall-windows-replace-a-rename-target-where-posix-rename-is-missing.patch) | [2ffda87](https://github.com/golang/go/commit/2ffda87f2dce71024f72ccff32cbfe29ee676bf8) (Go 1.25) |
| A junction or directory symlink to a file opens on Windows 7, 8 and 8.1 where Windows 10 refuses it. Links are held to their type on every Windows, so a program gets the same answer everywhere. | [0016](patches/0016-os-syscall-hold-a-link-to-its-type-where-the-kernel-follows-it-regardless.patch) | [b19851b](https://github.com/golang/go/commit/b19851bb60903511adf7ca885619c18ed7206d7b) (Go 1.28, and 1.26.9 and 1.27.2 by backport)<br>Kernel |
| `os.Root.Symlink` crashes the process on Windows 7. `DeviceIoControl` is given a place to store the byte count, which Windows 7 writes regardless. Upstream's own fix, not backported to Go 1.26. | [0019](patches/0019-internal-syscall-windows-pass-lpbytesreturned-when-setting-a-reparse-point.patch) | [26fdb07](https://github.com/golang/go/commit/26fdb07d4ce58885305283ba18960f582f4eaa73) (Go 1.25) |

### Race detector

| What it fixes | Patch | Upstream change |
| --- | --- | --- |
| A `-race` binary does not start on Windows 7. The race runtime's address wait functions are defined by the fork and emulated where the kernel lacks them, and the Windows 8 import library is no longer linked. | [0017](patches/0017-runtime-race-cmd-link-let-race-binaries-start-on-windows-7.patch) | [c3bea70](https://github.com/golang/go/commit/c3bea70d9b3b80ceb7733cd7bde0cdf0a1bfd0d0) (Go 1.19)<br>[cc82867](https://github.com/golang/go/commit/cc82867f6bf650e6b48a6e87849e4fdd5b94ef70) (Go 1.21) |

### The go command

| What it fixes | Patch | Upstream change |
| --- | --- | --- |
| `go get` in GOPATH mode is gone since Go 1.22. Restored, so `GO111MODULE=off go get` fetches into `GOPATH/src` and installs again. | [9999](patches/9999-cmd-go-restore-gopath-mode-go-get.patch) | [de4d503](https://github.com/golang/go/commit/de4d50316fb5c6d1529aa5377dc93b26021ee843) (Go 1.22) |

### Tests only

| What it fixes | Patch | Upstream change |
| --- | --- | --- |
| The VHD tests pass on Windows 7 without testing anything. The Hyper-V cmdlets they need arrived with Windows 8. They skip where the cmdlets are missing. | [T0002](patches/T0002-path-filepath-skip-the-vhd-tests-where-the-hyper-v-cmdlets-are-missing.patch) | [7986e26](https://github.com/golang/go/commit/7986e26a39e9df870886a9933107372f4e16ea4c) (Go 1.23) |
| A live TLS resumption test fails where the root store lacks the server's root. It skips when no dial verifies. | [T0003](patches/T0003-crypto-tls-skip-the-live-resumption-check-when-the-chain-cannot-be-verified.patch) | [b691a2e](https://github.com/golang/go/commit/b691a2edc7f5863f61a07c4a4f087eef1a15a704) (Go 1.27, and 1.26.0 by backport) |
| The system certificate verification tests fail where the root store lacks GTS Root R1. Those cases skip. | [T0004](patches/T0004-crypto-x509-skip-system-verify-cases-whose-root-the-store-lacks.patch) | [bb8f9a6](https://github.com/golang/go/commit/bb8f9a6ae66d742cb67b4ad444179905a537de00) (Go 1.21) |

## Test Results

Every fallback is coupled with tests, and the fork passes Go's test suite on each system below.

| System | Version | Toolchain | Go test suite |
| --- | --- | --- | --- |
| Windows 7 32-bit | 6.1.7600 | `windows_386` | [✅ Passed](https://glw7.purrsys.io/release/1.26/win7-386) |
| Windows 7 64-bit | 6.1.7600 | `windows_amd64` | [✅ Passed](https://glw7.purrsys.io/release/1.26/win7-amd64) |
| Windows 8 64-bit | 6.2.9200 | `windows_amd64` | [✅ Passed](https://glw7.purrsys.io/release/1.26/win8-amd64) |
| Windows 8.1 64-bit | 6.3.9600 | `windows_amd64` | [✅ Passed](https://glw7.purrsys.io/release/1.26/win81-amd64) |
| Windows 10 32-bit | 10.0.19045 | `windows_386` | [✅ Passed](https://glw7.purrsys.io/release/1.26/win10-386) |
| Windows 11 64-bit | 10.0.26300 | `windows_amd64` | [✅ Passed](https://glw7.purrsys.io/release/1.26/win11-amd64) |

On Windows 10 and 11 the fork matches stock Go.

32-bit builds are tested only on Windows 7 and 10. They behave the same as their 64-bit siblings on the same kernel.

## Download and Install

### Binary Distributions

<p align="center">
  <a href="https://glw7.purrsys.io/release/1.26"><img src="https://img.shields.io/github/v/release/thongtech/go-legacy-win7?style=for-the-badge&filter=v1.26*&sort=semver&display_name=tag&label=Latest%20release&color=00ADD8&logo=go&logoColor=white" alt="Latest release of the Go 1.26 line"></a>
  &nbsp;
  <a href="https://glw7.purrsys.io/release/1.25"><img src="https://img.shields.io/github/v/release/thongtech/go-legacy-win7?style=for-the-badge&filter=v1.25*&sort=semver&display_name=tag&label=Previous%20line&color=007D9C&logo=go&logoColor=white" alt="Latest release of the Go 1.25 line"></a>
</p>

Release archives do not include the tests and scripts. Clone the repo if you want to run them.

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
