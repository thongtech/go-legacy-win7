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
| **macOS** | Intel (amd64) | [go-legacy-win7-1.25.2-1.darwin_amd64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.2-1/go-legacy-win7-1.25.2-1.darwin_amd64.tar.gz) | `6ea2959ee45e512a4439d7224309b9fa660a7e0f175ebb6608c5ce5105a9bad9` |
| macOS | Apple (ARM64) | [go-legacy-win7-1.25.2-1.darwin_arm64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.2-1/go-legacy-win7-1.25.2-1.darwin_arm64.tar.gz) | `b0fd514b8f8bc8caa680b3dc609f905211b022f725717cfb11f93daf6fe9a3ac` |
| **Linux** | x86 (386) | [go-legacy-win7-1.25.2-1.linux_386.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.2-1/go-legacy-win7-1.25.2-1.linux_386.tar.gz) | `4411c1adad34da39ac294d358254cc740878674b89f46dcd3ec27493bb551405` |
| Linux | x64 (amd64) | [go-legacy-win7-1.25.2-1.linux_amd64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.2-1/go-legacy-win7-1.25.2-1.linux_amd64.tar.gz) | `e8bfa8a317078ec80f86ecc41c74eafe4434bf1552af0bf087b69b56463f4e97` |
| Linux | ARM (32‑bit) | [go-legacy-win7-1.25.2-1.linux_arm.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.2-1/go-legacy-win7-1.25.2-1.linux_arm.tar.gz) | `c061e450a5371001ba08f5e7adcc9e08b487ef119289bc128d91974691618537` |
| Linux | ARM64 | [go-legacy-win7-1.25.2-1.linux_arm64.tar.gz](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.2-1/go-legacy-win7-1.25.2-1.linux_arm64.tar.gz) | `164b47979ba8463daa0882156fa2930754b27adfd04de91d3cc3323120a9ff71` |
| **Windows** | x86 (386) | [go-legacy-win7-1.25.2-1.windows_386.zip](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.2-1/go-legacy-win7-1.25.2-1.windows_386.zip) | `a3327af3d1148fd11099ed07b3f4a55fb7cda0ce9102e0c4ee60124f3657991c` |
| Windows | x64 (amd64) | [go-legacy-win7-1.25.2-1.windows_amd64.zip](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.2-1/go-legacy-win7-1.25.2-1.windows_amd64.zip) | `eb64f8d878b0758c2e9d97d4e84a87373bec764178e8f07dfce62b6fe8f78221` |
| Windows | ARM64 | [go-legacy-win7-1.25.2-1.windows_arm64.zip](https://github.com/thongtech/go-legacy-win7/releases/download/v1.25.2-1/go-legacy-win7-1.25.2-1.windows_arm64.zip) | `329f30155f6d5e83532176a470e25aed3174bf0a1247fe027e4a53e8c8854e81` |

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
