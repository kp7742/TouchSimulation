package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/lunixbochs/struc"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

// InputDevice A Linux input device
type InputDevice struct {
	Name string
	Path string
	AbsX AbsInfo
	AbsY AbsInfo
	Slot AbsInfo
	File *os.File
}

//Grab the input device exclusively.
func (dev *InputDevice) Grab() error {
	return ioctl(dev.File.Fd(), EVIOCGRAB(), uintptr(1))
}

//Release a grabbed input device.
func (dev *InputDevice) Release() error {
	return ioctl(dev.File.Fd(), EVIOCGRAB(), uintptr(0))
}

//Fetch Active Input Devices
func getInputDevices() ([]*InputDevice, error) {
	paths, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, err
	}

	var ids []*InputDevice

	for _, path := range paths {
		if isCharDevice(path) {
			inDev, err := os.OpenFile(path, syscall.O_RDONLY|syscall.O_NONBLOCK, 0666)
			if err == nil {
				if hasSpecificAbs(inDev, absMtSlot) &&
					hasSpecificAbs(inDev, absMtTrackingId) &&
					hasSpecificAbs(inDev, absMtPositionX) &&
					hasSpecificAbs(inDev, absMtPositionY) &&
					hasSpecificProp(inDev, inputPropDirect) {
					absX, err := getAbsInfo(inDev, absMtPositionX)
					if err != nil {
						continue
					}

					absY, err := getAbsInfo(inDev, absMtPositionY)
					if err != nil {
						continue
					}

					slot, err := getAbsInfo(inDev, absMtSlot)
					if err != nil {
						continue
					}

					ids = append(ids, &InputDevice{
						AbsX: absX,
						AbsY: absY,
						Slot: slot,
						Path: path,
						File: inDev,
						Name: getDeviceName(inDev),
					})
				}
			}
		}
	}

	if ids != nil && len(ids) > 0 {
		return ids, nil
	} else {
		return nil, errors.New("devices are not found")
	}
}

//Determine if a path exist and is a character input device.
func isCharDevice(path string) bool {
	fi, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	}

	m := fi.Mode()
	if m&os.ModeCharDevice == 0 {
		return false
	}

	return true
}

//Determine if a event has specified Input Prop.
func hasSpecificProp(f *os.File, key int) bool {
	propBits := new([inputPropCnt / 8]byte)

	err := ioctl(f.Fd(), EVIOCGPROP(), uintptr(unsafe.Pointer(propBits)))
	if err != nil {
		return false
	}

	return propBits[key/8]&(1<<uint(key%8)) != 0
}

//Determine if a event has specified Abs Key.
func hasSpecificAbs(f *os.File, key int) bool {
	absBits := new([absCnt / 8]byte)

	err := ioctl(f.Fd(), EVIOCGBIT(evAbs, absMax), uintptr(unsafe.Pointer(absBits)))
	if err != nil {
		return false
	}

	return absBits[key/8]&(1<<uint(key%8)) != 0
}

//Read Input Device's ABS Data
func getAbsInfo(f *os.File, key int) (AbsInfo, error) {
	absData := AbsInfo{}

	err := ioctl(f.Fd(), EVIOCGABS(key), uintptr(unsafe.Pointer(&absData)))
	if err != nil {
		return AbsInfo{}, err
	}

	return absData, nil
}

//Read Event's Device Name
func getDeviceName(f *os.File) string {
	name := new([uinputMaxNameSize]byte)

	err := ioctl(f.Fd(), EVIOCGNAME(), uintptr(unsafe.Pointer(name)))
	if err != nil {
		return "Default"
	}

	idx := bytes.IndexByte(name[:], 0)

	return string(name[:idx])
}

//Find Touch Device and Create new UInput
func newUInputDevice(inputDev *InputDevice) (*InputDevice, error) {
	//Open UInput
	deviceFile, err := os.OpenFile("/dev/uinput", syscall.O_WRONLY|syscall.O_NONBLOCK, 0660)
	if err != nil {
		return nil, err
	}

	//Setup Touch Value
	err = ioctl(deviceFile.Fd(), UISETEVBIT(), evKey)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}
	err = ioctl(deviceFile.Fd(), UISETKEYBIT(), btnTouch)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}

	//Setup Touch Params
	err = ioctl(deviceFile.Fd(), UISETEVBIT(), evAbs)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}
	err = ioctl(deviceFile.Fd(), UISETABSBIT(), absMtPositionX)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}
	err = ioctl(deviceFile.Fd(), UISETABSBIT(), absMtPositionY)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}
	err = ioctl(deviceFile.Fd(), UISETABSBIT(), absMtTrackingId)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}
	err = ioctl(deviceFile.Fd(), UISETPROPBIT(), inputPropDirect)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}

	//Setup User Device
	var absMin [absCnt]int32
	absMin[absMtPositionX] = inputDev.AbsX.Minimum
	absMin[absMtPositionY] = inputDev.AbsY.Minimum
	absMin[absMtTrackingId] = 0

	var absMax [absCnt]int32
	absMax[absMtPositionX] = inputDev.AbsX.Maximum
	absMax[absMtPositionY] = inputDev.AbsY.Maximum
	absMax[absMtTrackingId] = inputDev.Slot.Maximum

	newDeviceName := randStringBytes(7)
	newVendor := randUInt16Num(0x2000)
	newProduct := randUInt16Num(0x2000)
	newVersion := randUInt16Num(0x20)

	uiDev := UinputUserDev{
		Name: toUInputName([]byte(newDeviceName)),
		ID: InputID{
			BusType: 0,
			Vendor:  newVendor,
			Product: newProduct,
			Version: newVersion,
		},
		EffectsMax: 0,
		AbsMax:     absMax,
		AbsMin:     absMin,
		AbsFuzz:    [absCnt]int32{},
		AbsFlat:    [absCnt]int32{},
	}

	//Write to Input Sub-System
	_, err = deviceFile.Write(uInputDevToBytes(uiDev))
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}

	//Declare Input Device
	err = createDevice(deviceFile)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}

	//Stop Primary Touch Device
	err = inputDev.Grab()
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}

	time.Sleep(time.Millisecond * 200)

	return &InputDevice{
		File: deviceFile,
		Name: newDeviceName,
	}, nil
}

func toUInputName(name []byte) [uinputMaxNameSize]byte {
	var fixedSizeName [uinputMaxNameSize]byte
	copy(fixedSizeName[:], name)
	return fixedSizeName
}

func uInputDevToBytes(uiDev UinputUserDev) []byte {
	var buf bytes.Buffer
	_ = struc.PackWithOptions(&buf, &uiDev, &struc.Options{Order: binary.LittleEndian})
	return buf.Bytes()
}

func createDevice(f *os.File) (err error) {
	return ioctl(f.Fd(), UIDEVCREATE(), uintptr(0))
}

func releaseDevice(f *os.File) (err error) {
	return ioctl(f.Fd(), UIDEVDESTROY(), uintptr(0))
}
