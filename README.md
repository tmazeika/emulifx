# Emulifx
LIFX device emulator.

This graphical program attempts to replicate an actual LIFX device on the network. It can be controlled from the LIFX mobile app, but due to what seems to be undocumented protocol messages being passed from actual LIFX devices to the mobile app (both proprietary), it cannot be claimed by the app. However, you may still control the colors and such from your phone, but you won't be able to add it to scenes or schedules.

**Contents:**
- [Installation](#installation)

## Installation
If you have Go installed and `$GOPATH/bin` in your path, run `go get -u github.com/lifx-tools/emulifx` and the `emulifx` binary will be available. Otherwise, [download the latest release](https://github.com/lifx-tools/emulifx/releases) for your platform, unarchive it, and move the binary to some location in your path (try `/usr/bin/`). The GUI portion of this program requires CGO, meaning binaries will only be distributed for a limited number of platforms.

Should you have issues with GLFW or OpenGL, see the read-me's for the [appropriate repository in go-gl](https://github.com/go-gl).
