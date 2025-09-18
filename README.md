# The Go Programming Language

**go-legacy-win7** is a fork of the Go programming language that maintains support for Windows 7 and Windows Server 2008 R2, and allows for deprecated `go get` behaviour. This project aims to provide a stable Go environment for users who need to support legacy Windows systems or prefer the traditional Go workflow.

![Gopher image](https://golang.org/doc/gopher/fiveyears.jpg)
_Gopher image by [Renee French][rf], licensed under [Creative Commons 4.0 Attribution licence][cc4-by]._

## Differences from Upstream Go

1. **Windows 7 and Windows Server 2008 R2 Support**  
   While the official Go project has dropped support for Windows 7 and Windows Server 2008 R2, this fork maintains compatibility with these legacy Windows systems.

   Tested on Windows 7 RTM (build 7600) — no updates required — through Windows 11 24H2

2. **Classic `go get` Behaviour**  
   This fork allows for the deprecated `go get` behaviour when `GO111MODULE` is set to "off" or "auto". This means:

   - In `GOPATH/src`, `go get` and `go install` can operate in `GOPATH` mode.
   - Outside of `GOPATH/src`, these commands can use module-aware mode when appropriate.

3. **Compatibility Notes**  
   Please be aware that some newer Go features may not be fully compatible with Windows 7 or Windows Server 2008 R2. We try to maintain as much functionality as possible, but some limitations may exist.

## Changes in Each Release

Current release includes the following modifications:

- Switched back to RtlGenRandom from ProcessPrng, which breaks Win7/2008R2 (reverted [693def1](https://github.com/golang/go/commit/693def151adff1af707d82d28f55dba81ceb08e1))
- Added back LoadLibraryA fallback to load system libraries (reverted [a17d959](https://github.com/golang/go/commit/a17d959debdb04cd550016a3501dd09d50cd62e7))
- Added back sysSocket fallback for socket syscalls (reverted [7c1157f](https://github.com/golang/go/commit/7c1157f9544922e96945196b47b95664b1e39108))
- Added back Windows 7 console handle workaround (reverted [48042aa](https://github.com/golang/go/commit/48042aa09c2f878c4faa576948b07fe625c4707a))
- Added back 5ms sleep on Windows 7/8 in (\*Process).Wait (reverted [f0894a0](https://github.com/golang/go/commit/f0894a00f4b756d4b9b4078af2e686b359493583))
- Restored deprecated `go get` behavior for use outside modules (reverted [de4d503](https://github.com/golang/go/commit/de4d50316fb5c6d1529aa5377dc93b26021ee843))
- Reverted to the previous `removeall_noat` variant for Windows (fixed [issue #2](https://github.com/thongtech/go-legacy-win7/issues/2))
- Rolled back `race_windows.syso` to the previous compatible version (fixed [issue #3](https://github.com/thongtech/go-legacy-win7/issues/3))
- Includes all improvements and bug fixes from the corresponding upstream Go release

The Windows binary provided here also supports Windows 7 and Windows Server 2008 R2

## Download and Install

### Binary Distributions

| OS | Architecture | Filename | SHA‑256 Hash |
|----|--------------|----------|--------------|
| **macOS** | Intel (amd64) | [go-legacy-win7-1.25.1-2.darwin_amd64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.1-2/go-legacy-win7-1.25.1-2.darwin_amd64.tar.gz) | `e71c71a063f734d5154d16c33d55e92d332ddfa8487369c3f76b297056f739bc` |
| macOS | Apple (ARM64) | [go-legacy-win7-1.25.1-2.darwin_arm64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.1-2/go-legacy-win7-1.25.1-2.darwin_arm64.tar.gz) | `25b0f28307aeb20a8a9cdebb6f360de00db98bde0ef5b7cc7a75556c0dfdf488` |
| **Linux** | x86 (386) | [go-legacy-win7-1.25.1-2.linux_386.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.1-2/go-legacy-win7-1.25.1-2.linux_386.tar.gz) | `a9751b2a55ef4d85aa8b113fac74e2a7ef564866ff5d30df479efd230455acfc` |
| Linux | x64 (amd64) | [go-legacy-win7-1.25.1-2.linux_amd64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.1-2/go-legacy-win7-1.25.1-2.linux_amd64.tar.gz) | `811c0c35afd64d290f74c5fa1ffaf203032cc9f5ca264a9b5405d4e51a952e67` |
| Linux | ARM (32‑bit) | [go-legacy-win7-1.25.1-2.linux_arm.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.1-2/go-legacy-win7-1.25.1-2.linux_arm.tar.gz) | `d18912582ddae96dd38c0a37b0a4b6e9ad94035f4768c4b0fa24f5122d549edb` |
| Linux | ARM64 | [go-legacy-win7-1.25.1-2.linux_arm64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.1-2/go-legacy-win7-1.25.1-2.linux_arm64.tar.gz) | `4b55dc9898af4fccbb5ce31da3de78e881c6ba2883d87b7d59f810b1a971f450` |
| **Windows** | x86 (386) | [go-legacy-win7-1.25.1-2.windows_386.zip](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.1-2/go-legacy-win7-1.25.1-2.windows_386.zip) | `caee0d6e4e323d98e63fcaa42bc563986fe47b3f4f0dc76aee0d1b7fd7ee9ac3` |
| Windows | x64 (amd64) | [go-legacy-win7-1.25.1-2.windows_amd64.zip](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.1-2/go-legacy-win7-1.25.1-2.windows_amd64.zip) | `4a6eaf54116e20aeb1c777551deafdea3c8c3d82c869441703b68e063b93e808` |
| Windows | ARM64 | [go-legacy-win7-1.25.1-2.windows_arm64.zip](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.1-2/go-legacy-win7-1.25.1-2.windows_arm64.zip) | `b4563f51d9459040fb09f291ffa4d1dea0962f66b961a36d6d102245f081fabe` |

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
