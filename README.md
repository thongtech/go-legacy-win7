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

> Note: The table below shows the latest Go stable version. For previous versions, including other stable releases, visit the [Releases](https://github.com/thongtech/go-legacy-win7/releases) page and select the one matching your desired Go version.

| OS | Architecture | Filename | SHA‑256 Hash |
|----|--------------|----------|--------------|
| macOS | Intel (amd64) | [go-legacy-win7-1.26.5-1.darwin_amd64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.26.5-1/go-legacy-win7-1.26.5-1.darwin_amd64.tar.gz) | `4b6c0b32677b6de90c9520d433fa02532e18892d43c357d2ad8434debc36e874` |
| **macOS** | **Apple (ARM64)** | [go-legacy-win7-1.26.5-1.darwin_arm64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.26.5-1/go-legacy-win7-1.26.5-1.darwin_arm64.tar.gz) | `461bf3c54d62b3c9f819c3808c8887503e407c74caa1518501ce4d5ca34986f6` |
| Linux | x86 (386) | [go-legacy-win7-1.26.5-1.linux_386.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.26.5-1/go-legacy-win7-1.26.5-1.linux_386.tar.gz) | `106b76f81e95c35ad435fb345c179a592923da028df983f76c975ac2e0c4da0f` |
| **Linux** | **x64 (amd64)** | [go-legacy-win7-1.26.5-1.linux_amd64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.26.5-1/go-legacy-win7-1.26.5-1.linux_amd64.tar.gz) | `4204dcd95218f61f4ffee51d0deef05a227228ae1f494755e855414271d7c4af` |
| Linux | ARM (32‑bit) | [go-legacy-win7-1.26.5-1.linux_arm.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.26.5-1/go-legacy-win7-1.26.5-1.linux_arm.tar.gz) | `857d49b27e8d53acfe131aa14eafaaff4fbb2002effa6bc5bc6c86ca92a266ef` |
| Linux | ARM64 | [go-legacy-win7-1.26.5-1.linux_arm64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.26.5-1/go-legacy-win7-1.26.5-1.linux_arm64.tar.gz) | `126523010148d9c942c8bfba049d0a88c32a2a8047a3d56018549d5394a5eda2` |
| Windows | x86 (386) | [go-legacy-win7-1.26.5-1.windows_386.zip](https://github.com/thongtech/go-legacy-win7/releases/download/v1.26.5-1/go-legacy-win7-1.26.5-1.windows_386.zip) | `2bcd0417a58d569371382f3a98d29787eb9cec271a205c8fab8e3e36f2b703dc` |
| **Windows** | **x64 (amd64)** | [go-legacy-win7-1.26.5-1.windows_amd64.zip](https://github.com/thongtech/go-legacy-win7/releases/download/v1.26.5-1/go-legacy-win7-1.26.5-1.windows_amd64.zip) | `c9d0c79dc2b408a4ea580b62a3d093a4219f9ff95316ef891dc987827e6900e3` |
| Windows | x64 (amd64) - Race | [go-legacy-win7-1.26.5-1-race.windows_amd64.zip](https://github.com/thongtech/go-legacy-win7/releases/download/v1.26.5-1/go-legacy-win7-1.26.5-1-race.windows_amd64.zip) | `1f0412b3a8995dab7ad2559cfd65a1449b542273e5cbfc2dab5eb91ba6d0630a` |
| Windows | ARM64 | [go-legacy-win7-1.26.5-1.windows_arm64.zip](https://github.com/thongtech/go-legacy-win7/releases/download/v1.26.5-1/go-legacy-win7-1.26.5-1.windows_arm64.zip) | `3104cea0a9c0ed26573c0422901650f978862108a6410fce5b7b535765e73dad` |

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

## Contributing

Feedback and issue reports are welcome, and we encourage you to open pull requests to contribute to the project. We appreciate your help!

Note that the Go project uses the issue tracker for bug reports and
proposals only. See https://go.dev/wiki/Questions for a list of
places to ask questions about the Go language.

[rf]: https://reneefrench.blogspot.com/
[cc4-by]: https://creativecommons.org/licenses/by/4.0/
