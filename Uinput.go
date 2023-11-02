package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"github.com/lunixbochs/struc"
)

// InputDevice A Linux input device
type InputDevice struct {
	Name           string
	Path           string
	Slots          int32
	Version        int32
	TouchXMin      int32
	TouchXMax      int32
	TouchYMin      int32
	TouchYMax      int32
	Grabed         bool
	hasTouchMajor  bool
	hasTouchMinor  bool
	hasWidthMajor  bool
	hasWidthMinor  bool
	hasOrientation bool
	hasPressure    bool
	Dbits          *[evCnt / 8]byte
	AbsBits        *[absCnt / 8]byte
	KeyBits        *[keyCnt / 8]byte
	PropBits       *[inputPropCnt / 8]byte
	AbsInfos       map[int]AbsInfo
	IID            InputID
	File           *os.File
}

// Grab the input device exclusively.
func (dev *InputDevice) Grab() error {
	dev.Grabed = true
	return ioctl(dev.File.Fd(), EVIOCGRAB(), uintptr(1))
}

// Release a grabbed input device.
func (dev *InputDevice) Release() error {
	dev.Grabed = false
	return ioctl(dev.File.Fd(), EVIOCGRAB(), uintptr(0))
}

// Determine if input device has specified Abs Key.
func (dev *InputDevice) hasAbs(key int) bool {
	return dev.AbsBits[key/8]&(1<<uint(key%8)) != 0
}

// Fetch Active Input Devices
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
				// Read Ev data
				dBits := new([evCnt / 8]byte)
				err := ioctl(inDev.Fd(), EVIOCGBIT(0, evMax), uintptr(unsafe.Pointer(dBits)))
				if err != nil {
					continue
				}

				// Read Abs data
				absBits := new([absCnt / 8]byte)
				err = ioctl(inDev.Fd(), EVIOCGBIT(evAbs, absMax), uintptr(unsafe.Pointer(absBits)))
				if err != nil {
					continue
				}

				// Read Prop data
				propBits := new([inputPropCnt / 8]byte)
				err = ioctl(inDev.Fd(), EVIOCGPROP(), uintptr(unsafe.Pointer(propBits)))
				if err != nil {
					continue
				}

				// Read Key data
				keyBits := new([keyCnt / 8]byte)
				err = ioctl(inDev.Fd(), EVIOCGBIT(evKey, keyMax), uintptr(unsafe.Pointer(keyBits)))
				if err != nil {
					continue
				}

				// Devices with ABS_MT_SLOT - 1 aren't MT devices, libevdev:libevdev.c#L319
				if !hasSpecificAbs(absBits, absMtSlot-1) &&
					hasSpecificAbs(absBits, absMtSlot) &&
					hasSpecificAbs(absBits, absMtTrackingId) &&
					hasSpecificAbs(absBits, absMtPositionX) &&
					hasSpecificAbs(absBits, absMtPositionY) &&
					hasSpecificProp(propBits, inputPropDirect) &&
					hasSpecificKey(keyBits, btnTouch) {

					id := &InputDevice{
						Path:     path,
						File:     inDev,
						Dbits:    dBits,
						AbsBits:  absBits,
						KeyBits:  keyBits,
						PropBits: propBits,
					}

					// Read all AbsInfos
					for i := 0; i <= absMax; i++ {
						if !hasSpecificAbs(absBits, i) {
							continue
						}

						absInfo, err := getAbsInfo(inDev, i)
						if err != nil {
							continue
						}

						switch i {
						case absMtSlot:
							id.Slots = absInfo.Maximum + 1
							break
						case absMtTrackingId:
							if absInfo.Maximum == absInfo.Minimum {
								absInfo.Minimum = -1
								absInfo.Maximum = 0xFFFF
							}
							break
						case absMtPositionX:
							id.TouchXMin = absInfo.Minimum
							id.TouchXMax = absInfo.Maximum - absInfo.Minimum + 1
							break
						case absMtPositionY:
							id.TouchYMin = absInfo.Minimum
							id.TouchYMax = absInfo.Maximum - absInfo.Minimum + 1
							break
						}

						if id.AbsInfos == nil {
							id.AbsInfos = make(map[int]AbsInfo)
						}
						id.AbsInfos[i] = absInfo
					}

					// Read InputID
					id.IID, err = getInputID(inDev)
					if err != nil {
						continue
					}

					// Read Driver Version
					err := ioctl(inDev.Fd(), EVIOCGVERSION(), uintptr(unsafe.Pointer(&id.Version)))
					if err != nil {
						continue
					}

					id.Name = getDeviceName(inDev)
					id.hasTouchMajor = id.hasAbs(absMtTouchMajor)
					id.hasTouchMinor = id.hasAbs(absMtTouchMinor)
					id.hasWidthMajor = id.hasAbs(absMtWidthMajor)
					id.hasWidthMinor = id.hasAbs(absMtWidthMinor)
					id.hasOrientation = id.hasAbs(absMtOrientation)
					id.hasPressure = id.hasAbs(absMtPressure)

					ids = append(ids, id)
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

// Determine if a path exist and is a character input device.
func isCharDevice(path string) bool {
	fi, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	}

	return fi.Mode()&os.ModeCharDevice != 0
}

// Determine if a evbits has specified Event Type.
func hasSpecificType(evbits *[96]byte, key int) bool {
	return evbits[key/8]&(1<<uint(key%8)) != 0
}

// Determine if a propbits has specified Input Prop.
func hasSpecificProp(propBits *[4]byte, key int) bool {
	return propBits[key/8]&(1<<uint(key%8)) != 0
}

// Determine if a absbits has specified Abs Key.
func hasSpecificAbs(absBits *[8]byte, key int) bool {
	return absBits[key/8]&(1<<uint(key%8)) != 0
}

// Determine if a keybits has specified Key.
func hasSpecificKey(keyBits *[96]byte, key int) bool {
	return keyBits[key/8]&(1<<uint(key%8)) != 0
}

// Read Input Device's ABS Data
func getAbsInfo(f *os.File, key int) (AbsInfo, error) {
	absData := AbsInfo{}

	err := ioctl(f.Fd(), EVIOCGABS(key), uintptr(unsafe.Pointer(&absData)))
	if err != nil {
		return AbsInfo{}, err
	}

	return absData, nil
}

// Read Input Device's InputID
func getInputID(f *os.File) (InputID, error) {
	input_id := InputID{}

	err := ioctl(f.Fd(), EVIOCGID(), uintptr(unsafe.Pointer(&input_id)))
	if err != nil {
		return InputID{}, err
	}

	return input_id, nil
}

// Read Event's Device Name
func getDeviceName(f *os.File) string {
	name := new([uinputMaxNameSize]byte)

	err := ioctl(f.Fd(), EVIOCGNAME(), uintptr(unsafe.Pointer(name)))
	if err != nil {
		return "Default"
	}

	idx := bytes.IndexByte(name[:], 0)

	return string(name[:idx])
}

// Create new Type-B UInput device with details given from Event device
func newTypeBDevSame(inputDev *InputDevice) (*InputDevice, error) {
	//Open UInput
	deviceFile, err := os.OpenFile("/dev/uinput", syscall.O_WRONLY|syscall.O_NONBLOCK, 0660)
	if err != nil {
		return nil, err
	}

	//Setup EV_KEY
	err = ioctl(deviceFile.Fd(), UISETEVBIT(), evKey)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}
	for i := 0; i <= keyMax; i++ {
		if !hasSpecificKey(inputDev.KeyBits, i) {
			continue
		}

		// btnTouch
		err = ioctl(deviceFile.Fd(), UISETKEYBIT(), uintptr(i))
		if err != nil {
			_ = releaseDevice(deviceFile)
			_ = deviceFile.Close()
			return nil, err
		}
	}

	//Setup EV_ABS
	err = ioctl(deviceFile.Fd(), UISETEVBIT(), evAbs)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}

	var absMins [absCnt]int32
	var absMaxs [absCnt]int32
	var absFuzz [absCnt]int32
	var absFlat [absCnt]int32

	for i := 0; i <= absMax; i++ {
		if !hasSpecificAbs(inputDev.AbsBits, i) {
			continue
		}

		err = ioctl(deviceFile.Fd(), UISETABSBIT(), uintptr(i))
		if err != nil {
			_ = releaseDevice(deviceFile)
			_ = deviceFile.Close()
			return nil, err
		}

		absMins[i] = inputDev.AbsInfos[i].Minimum
		absMaxs[i] = inputDev.AbsInfos[i].Maximum
		absFuzz[i] = inputDev.AbsInfos[i].Fuzz
		absFlat[i] = inputDev.AbsInfos[i].Flat
	}

	//Setup INPUT_PROP_DIRECT
	for i := 0; i <= inputPropMax; i++ {
		if !hasSpecificProp(inputDev.PropBits, i) {
			continue
		}

		// inputPropDirect
		err = ioctl(deviceFile.Fd(), UISETPROPBIT(), uintptr(i))
		if err != nil {
			_ = releaseDevice(deviceFile)
			_ = deviceFile.Close()
			return nil, err
		}
	}

	//Setup User Device
	effectsMax := uint32(0)
	if hasSpecificType(inputDev.Dbits, evFF) {
		effectsMax = 10
	}

	newDeviceName := inputDev.Name + "2"
	uiDev := UinputUserDev{
		Name: toUInputName([]byte(newDeviceName)),
		ID: InputID{
			BusType: inputDev.IID.BusType,
			Vendor:  inputDev.IID.Vendor,
			Product: inputDev.IID.Product,
			Version: inputDev.IID.Version,
		},
		EffectsMax: effectsMax,
		AbsMax:     absMaxs,
		AbsMin:     absMins,
		AbsFuzz:    absFuzz,
		AbsFlat:    absFlat,
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

// Create new Type-A UInput device with details given from Event device
func newTypeADevSame(inputDev *InputDevice) (*InputDevice, error) {
	//Open UInput
	deviceFile, err := os.OpenFile("/dev/uinput", syscall.O_WRONLY|syscall.O_NONBLOCK, 0660)
	if err != nil {
		return nil, err
	}

	//Setup EV_KEY
	err = ioctl(deviceFile.Fd(), UISETEVBIT(), evKey)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}
	for i := 0; i <= keyMax; i++ {
		if !hasSpecificKey(inputDev.KeyBits, i) {
			continue
		}

		// btnTouch
		err = ioctl(deviceFile.Fd(), UISETKEYBIT(), uintptr(i))
		if err != nil {
			_ = releaseDevice(deviceFile)
			_ = deviceFile.Close()
			return nil, err
		}
	}

	//Setup EV_ABS
	err = ioctl(deviceFile.Fd(), UISETEVBIT(), evAbs)
	if err != nil {
		_ = releaseDevice(deviceFile)
		_ = deviceFile.Close()
		return nil, err
	}

	var absMins [absCnt]int32
	var absMaxs [absCnt]int32
	var absFuzz [absCnt]int32
	var absFlat [absCnt]int32

	for i := 0; i <= absMax; i++ {
		if !hasSpecificAbs(inputDev.AbsBits, i) {
			continue
		}

		// Make Sure only TypeA enabled ABS got set
		// absMtPositionX, absMtPositionY, absMtTrackingId
		if i == absMtPositionX || i == absMtPositionY || i == absMtTrackingId {
			err = ioctl(deviceFile.Fd(), UISETABSBIT(), uintptr(i))
			if err != nil {
				_ = releaseDevice(deviceFile)
				_ = deviceFile.Close()
				return nil, err
			}

			absMins[i] = inputDev.AbsInfos[i].Minimum
			absMaxs[i] = inputDev.AbsInfos[i].Maximum
			absFuzz[i] = inputDev.AbsInfos[i].Fuzz
			absFlat[i] = inputDev.AbsInfos[i].Flat
		}
	}

	//Setup INPUT_PROP_DIRECT
	for i := 0; i <= inputPropMax; i++ {
		if !hasSpecificProp(inputDev.PropBits, i) {
			continue
		}

		// inputPropDirect
		err = ioctl(deviceFile.Fd(), UISETPROPBIT(), uintptr(i))
		if err != nil {
			_ = releaseDevice(deviceFile)
			_ = deviceFile.Close()
			return nil, err
		}
	}

	//Setup User Device
	effectsMax := uint32(0)
	if hasSpecificType(inputDev.Dbits, evFF) {
		effectsMax = 10
	}

	newDeviceName := inputDev.Name + "2"
	uiDev := UinputUserDev{
		Name: toUInputName([]byte(newDeviceName)),
		ID: InputID{
			BusType: inputDev.IID.BusType,
			Vendor:  inputDev.IID.Vendor,
			Product: inputDev.IID.Product,
			Version: inputDev.IID.Version,
		},
		EffectsMax: effectsMax,
		AbsMax:     absMaxs,
		AbsMin:     absMins,
		AbsFuzz:    absFuzz,
		AbsFlat:    absFlat,
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

// Create new Type-A UInput device with random details
func newTypeADevRandom(inputDev *InputDevice) (*InputDevice, error) {
	//Open UInput
	deviceFile, err := os.OpenFile("/dev/uinput", syscall.O_WRONLY|syscall.O_NONBLOCK, 0660)
	if err != nil {
		return nil, err
	}

	//Setup EV_KEY
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

	//Setup EV_ABS
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
	absMin[absMtPositionX] = inputDev.AbsInfos[absMtPositionX].Minimum
	absMin[absMtPositionY] = inputDev.AbsInfos[absMtPositionY].Minimum
	absMin[absMtTrackingId] = 0

	var absMax [absCnt]int32
	absMax[absMtPositionX] = inputDev.AbsInfos[absMtPositionX].Maximum
	absMax[absMtPositionY] = inputDev.AbsInfos[absMtPositionY].Maximum
	absMax[absMtTrackingId] = inputDev.Slots - 1

	newDeviceName := randStringBytes(7)
	newVendor := randUInt16Num(0x2000)
	newProduct := randUInt16Num(0x2000)
	newVersion := randUInt16Num(0x20)

	effectsMax := uint32(0)
	if hasSpecificType(inputDev.Dbits, evFF) {
		effectsMax = 10
	}

	uiDev := UinputUserDev{
		Name: toUInputName([]byte(newDeviceName)),
		ID: InputID{
			BusType: 0,
			Vendor:  newVendor,
			Product: newProduct,
			Version: newVersion,
		},
		EffectsMax: effectsMax,
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
