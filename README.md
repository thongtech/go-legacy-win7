# The Go Programming Language

**go-legacy-win7** is a fork of the Go programming language that maintains Windows 7 support and allows for deprecated `go get` behaviour. This project aims to provide a stable Go environment for users who need to support legacy Windows systems or prefer the traditional Go workflow.

![Gopher image](https://golang.org/doc/gopher/fiveyears.jpg)
_Gopher image by [Renee French][rf], licensed under [Creative Commons 4.0 Attribution licence][cc4-by]._

## Differences from Upstream Go

1. **Windows 7 Support**  
   Whilst the official Go project has dropped support for Windows 7, this fork maintains compatibility with Windows 7 systems.

2. **Classic `go get` Behaviour**  
   This fork allows for the deprecated `go get` behaviour when `GO111MODULE` is set to "off" or "auto". This means:

   - In `GOPATH/src`, `go get` and `go install` can operate in `GOPATH` mode.
   - Outside of `GOPATH/src`, these commands can use module-aware mode when appropriate.

3. **Compatibility Notes**  
   Please be aware that some newer Go features may not be fully compatible with Windows 7. We try to maintain as much functionality as possible, but some limitations may exist.

## Changes in Each Release

Every release includes the following modifications:

- Restored Windows 7 support by reverting [693def1](https://github.com/golang/go/commit/693def151adff1af707d82d28f55dba81ceb08e1)
  - The Windows binary provided here also supports Windows 7
- Restored deprecated `go get` behaviour for use outside modules (reverted [de4d503](https://github.com/golang/go/commit/de4d50316fb5c6d1529aa5377dc93b26021ee843))
- Includes all improvements and bug fixes from the corresponding upstream Go release

## Download and Install

### Binary Distributions

Binary distributions are **available at the [release page](https://github.com/thongtech/go-legacy-win7/releases)**.

#### Windows Installation

1. Download the `go-legacy-win7-<version>.windows-<arch>.zip` file.
2. Extract the ZIP to `C:\Go` (or any preferred location).
3. Add `C:\Go\bin` (or your chosen path) to the system `PATH`.
4. Add `%USERPROFILE%\go\bin` to the user `PATH`.
5. Add `%USERPROFILE%\go` as `GOPATH` to user variables.

#### macOS Installation

1. Download the appropriate `go-legacy-win7-<version>.darwin-<arch>.tar.gz` file.
2. Extract the archive to `/usr/local`:
   ```
   sudo tar -C /usr/local -xzf go-legacy-win7-<version>.darwin-<arch>.tar.gz
   ```
3. Add `/usr/local/go-legacy-win7/bin` to your PATH and set GOPATH:
   - For bash (if you're using bash):
     ```
     echo 'export PATH=$PATH:/usr/local/go-legacy-win7/bin:$HOME/go-legacy-win7/bin' >> ~/.bash_profile
     echo 'export GOPATH=$HOME/go' >> ~/.bash_profile
     source ~/.bash_profile
     ```
   - For zsh (default on macOS Catalina and later):
     ```
     echo 'export PATH=$PATH:/usr/local/go-legacy-win7/bin:$HOME/go-legacy-win7/bin' >> ~/.zshrc
     echo 'export GOPATH=$HOME/go' >> ~/.zshrc
     source ~/.zshrc
     ```

#### Linux Installation

1. Download the appropriate `go-legacy-win7-<version>.linux-<arch>.tar.gz` file.
2. Extract the archive to `/usr/local`:
   ```
   sudo tar -C /usr/local -xzf go-legacy-win7-<version>.linux-<arch>.tar.gz
   ```
3. Add `/usr/local/go-legacy-win7/bin` to your PATH and set GOPATH:
   - For bash (default on most Linux distributions):
     ```
     echo 'export PATH=$PATH:/usr/local/go-legacy-win7/bin:$HOME/go-legacy-win7/bin' >> ~/.bashrc
     echo 'export GOPATH=$HOME/go' >> ~/.bashrc
     source ~/.bashrc
     ```
   - For zsh (if you're using zsh):
     ```
     echo 'export PATH=$PATH:/usr/local/go-legacy-win7/bin:$HOME/go-legacy-win7/bin' >> ~/.zshrc
     echo 'export GOPATH=$HOME/go' >> ~/.zshrc
     source ~/.zshrc
     ```

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
