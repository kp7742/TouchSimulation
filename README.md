## Touch Simulation
Touch Simulation is program made in Golang to simulate Touch Input in android devices using Virtual Display with UInput interface of Android(Linux) kernel. 

## Progress
- Implemented in pure Golang(Without C bindings).
- Generate random data for uinput device.
- Bridges Type-B device to Type-A device.
- Simulate Original Touch Screen data.
- Support 1 Touch Simulation point.
- Test Program to check simulation.

## Notes
- Not every device support directly, Modification may need
- Need either root access or adb shell
	
## How to Build
- Clone this repo.
- Install Android NDK and Go Binaries, if not already.
- Open bash in project folder and Execute build.sh script.
- Output will generate in same folder.

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
