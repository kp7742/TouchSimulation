## Touch Simulation
Touch Simulation is program to simulate Touch Input in android devices using Virtual Display with UInput interface of Android(Linux) kernel.

There are 2 variants made in Golang and C++.

## Features
- Generate random data for uinput device.
- Bridges Type-B device to Type-A device.
- Simulate Original Touch Screen data.
- Support 1 Touch Simulation point.
- Test Program to check simulation.

## Notes
- Not every device support directly, Modification may need.
- Need either root access or adb shell.
	
## How to Build Go variant
- Clone this repo.
- Install Android NDK and Go Binaries, if not already.
- Open bash in project directory and Execute build.sh script.
- Output will be in bin directory.
- Precompiled Binaries: [HERE](https://github.com/kp7742/TouchSimulation/tree/main/bin/)

## How to Build C++ variant
- Clone this repo.
- Install Android NDK, if not already.
- Open Shell/CMD in C++ directory.
- Drag ndk-build from NDK in Shell or CMD and then Execute.
- Output will be in libs directory.
- Precompiled Binaries: [HERE](https://github.com/kp7742/TouchSimulation/tree/main/C++/libs/)

## Sources
- [Linux Multi Touch Protocol](https://www.kernel.org/doc/Documentation/input/multi-touch-protocol.txt)
- [Android Touch Devices](https://source.android.com/devices/input/touch-devices)
- [Linux UInput](https://www.kernel.org/doc/html/v4.12/input/uinput.html)

## Credits
- [uinput](https://github.com/bendahl/uinput): UInput Wrappers
- [go-evdev](https://github.com/holoplot/go-evdev): InputEvent Definitions
- [golang-evdev](https://github.com/dddpaul/golang-evdev): IOCTL Definitions
- [Golang-evdev](https://github.com/gvalkov/golang-evdev): EVDEV Implementation

## Technlogy Communication
> Email: patel.kuldip91@gmail.com
