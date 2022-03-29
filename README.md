# video-archiver

## Build instructions

### Linux

`make build`

### Windows

- `git submodule init && git submodule update`
- Get MSYS2 installed (to C drive): https://www.msys2.org/
- Get GTK installed within MSYS2: https://www.gtk.org/docs/installations/windows/#using-gtk-from-msys2-packages
  - `pacman -S mingw-w64-x86_64-{gtk3,glade}`
  - Fix pkgconfig bug? `sed -i -e 's/-Wl,-luuid/-luuid/g' /mingw64/lib/pkgconfig/gdk-3.0.pc`
- Get `go` installed within MSYS2: `pacman -S mingw-w64-x86_64-go`
- (Probably also want `git` installed within MSYS2: `pacman -S git`)
- `make build-windows`
