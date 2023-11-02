package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/lunixbochs/struc"
)

type TypeMode int

const (
	TYPEA TypeMode = iota
	TYPEB
)

const (
	fakeContact = 9
)

var (
	currMode TypeMode

	touchSend  = false
	touchStart = false

	displayWidth  int32
	displayHeight int32

	fakeTouchMajor  int32 = -1
	fakeTouchMinor  int32 = -1
	fakeWidthMajor  int32 = -1
	fakeWidthMinor  int32 = -1
	fakeOrientation int32 = -1
	fakePressure    int32 = -1

	syncChannel chan bool
	stopChannel chan bool

	touchDevice *InputDevice
	uInputTouch *InputDevice

	touchContactsA []TouchContactA
	touchContactsB []TouchContactB
)

///----------Touch Contacts-----------///

// TouchContact Touch Contact Struct
type TouchContactA struct {
	PosX   int32
	PosY   int32
	Active bool
}

// TouchContact Touch Contact Struct
type TouchContactB struct {
	TouchMajor  int32
	TouchMinor  int32
	WidthMajor  int32
	WidthMinor  int32
	Orientation int32
	PositionX   int32
	PositionY   int32
	TrackingId  int32
	Pressure    int32

	Active      bool
	TUpdate     bool
	TMAUpdate   bool
	TMIUpdate   bool
	WMAUpdate   bool
	WMIUpdate   bool
	OriUpdate   bool
	PosXUpdate  bool
	PosYUpdate  bool
	TrackUpdate bool
	PressUpdate bool
}

///----------Touch Management Interface-----------///

// Read Input Event from Input Device
func readInputEvent(f *os.File) (InputEvent, error) {
	event := InputEvent{}
	buffer := make([]byte, unsafe.Sizeof(InputEvent{}))

	_, err := f.Read(buffer)
	if err != nil {
		return event, err
	}

	err = binary.Read(bytes.NewBuffer(buffer), binary.LittleEndian, &event)
	if err != nil {
		return event, err
	}

	return event, nil
}

func inputEventToBytes(event InputEvent) []byte {
	var buf bytes.Buffer
	_ = struc.PackWithOptions(&buf, &event, &struc.Options{Order: binary.LittleEndian})
	return buf.Bytes()
}

// Write Input Event to Specified Fd
func writeEvent(f *os.File, Type, Code uint16, Value int32) {
	_, _ = f.Write(inputEventToBytes(InputEvent{
		Time: syscall.Timeval{
			Sec:  0,
			Usec: 0,
		},
		Type:  Type,
		Code:  Code,
		Value: Value,
	}))
}

// Reading Touch Inputs from TypeA event
func eventReaderA() {
	var currSlot int32 = 0

	fmt.Printf("-------------------------------------\n")

	for {
		select {
		case <-stopChannel:
			return
		default:
		}

		inputEvent, err := readInputEvent(touchDevice.File)
		if err != nil {
			fmt.Printf("input read error\n")
			break
		}

		hasSyn := false

		switch inputEvent.Type {
		case evSyn:
			if inputEvent.Code == synReport {
				hasSyn = true
				fmt.Printf("SYN_REPORT\n")
			}
			break
		case evKey:
			if inputEvent.Code == btnTouch {
				touchType := "UP"
				if inputEvent.Value == 1 {
					touchType = "DOWN"
				}
				fmt.Printf("BTN_TOUCH: %s\n", touchType)
			}
			break
		case evAbs:
			switch inputEvent.Code {
			case absMtSlot:
				currSlot = inputEvent.Value
				fmt.Printf("ABS_MT_SLOT: %d\n", inputEvent.Value)
				break
			case absMtTrackingId:
				touchContactsA[currSlot].Active = inputEvent.Value != -1
				fmt.Printf("ABS_MT_TRACKING_ID: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtPositionX:
				touchContactsA[currSlot].PosX = inputEvent.Value
				fmt.Printf("ABS_MT_POSITION_X: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtPositionY:
				touchContactsA[currSlot].PosY = inputEvent.Value
				fmt.Printf("ABS_MT_POSITION_Y: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			}
			break
		}

		if hasSyn {
			syncChannel <- true
			fmt.Printf("-------------------------------------\n")
		}
	}
}

// Writing Touch Inputs to TypeA event
func eventDispatcherA() {
	var isBtnDown bool = false

	for {
		select {
		case <-stopChannel:
			return
		default:
		}

		select {
		case <-syncChannel:
			{
				nextSlot := 0

				for idx, contact := range touchContactsA {
					if contact.Active && contact.PosX > 0 && contact.PosY > 0 {
						writeEvent(uInputTouch.File, evAbs, absMtPositionX, contact.PosX)
						writeEvent(uInputTouch.File, evAbs, absMtPositionY, contact.PosY)
						writeEvent(uInputTouch.File, evAbs, absMtTrackingId, int32(idx))
						writeEvent(uInputTouch.File, evSyn, synMtReport, 0)

						nextSlot++
					}
				}

				if nextSlot == 0 && isBtnDown { //Button Up
					isBtnDown = false
					writeEvent(uInputTouch.File, evSyn, synMtReport, 0)
					writeEvent(uInputTouch.File, evKey, btnTouch, 0)
				} else if nextSlot > 0 && !isBtnDown { //Button Down
					isBtnDown = true
					writeEvent(uInputTouch.File, evKey, btnTouch, 1)
				}

				writeEvent(uInputTouch.File, evSyn, synReport, 0)
			}
		default:
		}
	}
}

// Reading Touch Inputs from TypeB event
func eventReaderB() {
	var currSlot int32 = 0

	fmt.Printf("-------------------------------------\n")

	for {
		select {
		case <-stopChannel:
			return
		default:
		}

		inputEvent, err := readInputEvent(touchDevice.File)
		if err != nil {
			fmt.Printf("input read error\n")
			break
		}

		hasSyn := false

		switch inputEvent.Type {
		case evSyn:
			if inputEvent.Code == synReport {
				hasSyn = true
				fmt.Printf("SYN_REPORT\n")
			}
			break
		case evKey:
			if inputEvent.Code == btnTouch {
				touchType := "UP"
				if inputEvent.Value == 1 {
					touchType = "DOWN"
				}
				fmt.Printf("BTN_TOUCH: %s\n", touchType)
			}
			break
		case evAbs:
			switch inputEvent.Code {
			case absMtSlot:
				currSlot = inputEvent.Value
				fmt.Printf("ABS_MT_SLOT: %d\n", inputEvent.Value)
				break
			case absMtTouchMajor:
				// The length of the major axis of the contact. The length should be given in surface units.
				// If the surface has an X times Y resolution, the largest possible value of ABS_MT_TOUCH_MAJOR is sqrt(X^2 + Y^2), the diagonal
				if touchContactsB[currSlot].Active {
					touchContactsB[currSlot].TUpdate = true
					touchContactsB[currSlot].TMAUpdate = true
					touchContactsB[currSlot].TouchMajor = inputEvent.Value
				}
				fmt.Printf("ABS_MT_TOUCH_MAJOR: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtTouchMinor:
				// The length, in surface units, of the minor axis of the contact. If the contact is circular, this event can be omitted
				if touchContactsB[currSlot].Active {
					touchContactsB[currSlot].TUpdate = true
					touchContactsB[currSlot].TMIUpdate = true
					touchContactsB[currSlot].TouchMinor = inputEvent.Value
				}
				fmt.Printf("ABS_MT_TOUCH_MINOR: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtWidthMajor:
				// The length, in surface units, of the major axis of the approaching tool. This should be understood as the size of the tool itself.
				// The orientation of the contact and the approaching tool are assumed to be the same
				if touchContactsB[currSlot].Active {
					touchContactsB[currSlot].TUpdate = true
					touchContactsB[currSlot].WMAUpdate = true
					touchContactsB[currSlot].WidthMajor = inputEvent.Value
				}
				fmt.Printf("ABS_MT_WIDTH_MAJOR: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtWidthMinor:
				// The length, in surface units, of the minor axis of the approaching tool. Omit if circular [4].
				// The above four values can be used to derive additional information about the contact.
				// The ratio ABS_MT_TOUCH_MAJOR / ABS_MT_WIDTH_MAJOR approximates the notion of pressure.
				// The fingers of the hand and the palm all have different characteristic widths.
				if touchContactsB[currSlot].Active {
					touchContactsB[currSlot].TUpdate = true
					touchContactsB[currSlot].WMIUpdate = true
					touchContactsB[currSlot].WidthMinor = inputEvent.Value
				}
				fmt.Printf("ABS_MT_WIDTH_MINOR: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtOrientation:
				// The orientation of the touching ellipse. The value should describe a signed quarter of a revolution clockwise around the touch center.
				// The signed value range is arbitrary, but zero should be returned for an ellipse aligned with the Y axis (north) of the surface,
				// a negative value when the ellipse is turned to the left, and a positive value when the ellipse is turned to the right.
				// When aligned with the X axis in the positive direction, the range max should be returned; when aligned with the X axis in the negative direction,
				// the range -max should be returned.
				// Touch ellipsis are symmetrical by default. For devices capable of true 360 degree orientation, the reported orientation must exceed the range max
				// to indicate more than a quarter of a revolution. For an upside-down finger, range max * 2 should be returned.
				// Orientation can be omitted if the touch area is circular, or if the information is not available in the kernel driver.
				// Partial orientation support is possible if the device can distinguish between the two axis, but not (uniquely) any values in between.
				// In such cases, the range of ABS_MT_ORIENTATION should be [0, 1] [4].
				if touchContactsB[currSlot].Active {
					touchContactsB[currSlot].TUpdate = true
					touchContactsB[currSlot].OriUpdate = true
					touchContactsB[currSlot].Orientation = inputEvent.Value
				}
				fmt.Printf("ABS_MT_ORIENTATION: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtPositionX:
				// The surface X coordinate of the center of the touching ellipse.
				if touchContactsB[currSlot].Active {
					touchContactsB[currSlot].TUpdate = true
					touchContactsB[currSlot].PosXUpdate = true
					touchContactsB[currSlot].PositionX = inputEvent.Value
				}
				fmt.Printf("ABS_MT_POSITION_X: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtPositionY:
				// The surface Y coordinate of the center of the touching ellipse.
				if touchContactsB[currSlot].Active {
					touchContactsB[currSlot].TUpdate = true
					touchContactsB[currSlot].PosYUpdate = true
					touchContactsB[currSlot].PositionY = inputEvent.Value
				}
				fmt.Printf("ABS_MT_POSITION_Y: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtToolType:
				// The type of approaching tool. A lot of kernel drivers cannot distinguish between different tool types, such as a finger or a pen.
				// In such cases, the event should be omitted.
				// The protocol currently supports MT_TOOL_FINGER, MT_TOOL_PEN, and MT_TOOL_PALM [2]. For type B devices, this event is handled by input core;
				// drivers should instead use input_mt_report_slot_state(). A contact’s ABS_MT_TOOL_TYPE may change over time while still touching the device,
				// because the firmware may not be able to determine which tool is being used when it first appears.
				fmt.Printf("ABS_MT_TOOL_TYPE: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtBlobId:
				// The BLOB_ID groups several packets together into one arbitrarily shaped contact. The sequence of points forms a polygon which defines the shape of the contact.
				// This is a low-level anonymous grouping for type A devices, and should not be confused with the high-level trackingID [5].
				// Most type A devices do not have blob capability, so drivers can safely omit this event.
				fmt.Printf("ABS_MT_BLOB_ID: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtTrackingId:
				// The TRACKING_ID identifies an initiated contact throughout its life cycle [5].
				// The value range of the TRACKING_ID should be large enough to ensure unique identification of a contact maintained over an extended period of time.
				// For type B devices, this event is handled by input core; drivers should instead use input_mt_report_slot_state().
				touchContactsB[currSlot].TUpdate = true
				touchContactsB[currSlot].TrackUpdate = true
				touchContactsB[currSlot].TrackingId = inputEvent.Value
				touchContactsB[currSlot].Active = inputEvent.Value != -1
				fmt.Printf("ABS_MT_TRACKING_ID: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtPressure:
				// The pressure, in arbitrary units, on the contact area. May be used instead of TOUCH and WIDTH for pressure-based devices
				// or any device with a spatial signal intensity distribution.
				if touchContactsB[currSlot].Active {
					touchContactsB[currSlot].TUpdate = true
					touchContactsB[currSlot].PressUpdate = true
					touchContactsB[currSlot].Pressure = inputEvent.Value
				}
				fmt.Printf("ABS_MT_PRESSURE: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtDistance:
				// The distance, in surface units, between the contact and the surface. Zero distance means the contact is touching the surface.
				// A positive number means the contact is hovering above the surface.
				fmt.Printf("ABS_MT_DISTANCE: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtToolX:
				// The surface X coordinate of the center of the approaching tool. Omit if the device cannot distinguish between the intended touch point and the tool itself.
				fmt.Printf("ABS_MT_TOOL_X: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtToolY:
				// The surface Y coordinate of the center of the approaching tool. Omit if the device cannot distinguish between the intended touch point and the tool itself.
				// The four position values can be used to separate the position of the touch from the position of the tool.
				// If both positions are present, the major tool axis points towards the touch point [1]. Otherwise, the tool axes are aligned with the touch axes.
				fmt.Printf("ABS_MT_TOOL_Y: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			}
			break
		}

		if hasSyn {
			syncChannel <- true
			fmt.Printf("-------------------------------------\n")
		}
	}
}

// Writing Touch Inputs to TypeB device
func eventDispatcherB() {
	var isBtnDown bool = false

	for {
		select {
		case <-stopChannel:
			return
		default:
		}

		select {
		case <-syncChannel:
			{
				activeSlots := 0

				for idx, contact := range touchContactsB {
					if contact.Active {
						activeSlots++

						writeEvent(uInputTouch.File, evAbs, absMtSlot, int32(idx))

						if contact.TUpdate {
							if contact.TrackUpdate {
								writeEvent(uInputTouch.File, evAbs, absMtTrackingId, contact.TrackingId)
								touchContactsB[idx].TrackUpdate = false
							}

							if contact.PosXUpdate {
								writeEvent(uInputTouch.File, evAbs, absMtPositionX, contact.PositionX)
								touchContactsB[idx].PosXUpdate = false
							}

							if contact.PosYUpdate {
								writeEvent(uInputTouch.File, evAbs, absMtPositionY, contact.PositionY)
								touchContactsB[idx].PosYUpdate = false
							}

							if contact.TMAUpdate {
								writeEvent(uInputTouch.File, evAbs, absMtTouchMajor, contact.TouchMajor)
								touchContactsB[idx].TMAUpdate = false
							}

							if contact.TMIUpdate {
								writeEvent(uInputTouch.File, evAbs, absMtTouchMinor, contact.TouchMinor)
								touchContactsB[idx].TMIUpdate = false
							}

							if contact.WMAUpdate {
								writeEvent(uInputTouch.File, evAbs, absMtWidthMajor, contact.WidthMajor)
								touchContactsB[idx].WMAUpdate = false
							}

							if contact.WMIUpdate {
								writeEvent(uInputTouch.File, evAbs, absMtWidthMinor, contact.WidthMinor)
								touchContactsB[idx].WMIUpdate = false
							}

							if contact.PressUpdate {
								writeEvent(uInputTouch.File, evAbs, absMtPressure, contact.Pressure)
								touchContactsB[idx].PressUpdate = false
							}

							if contact.OriUpdate {
								writeEvent(uInputTouch.File, evAbs, absMtOrientation, contact.Orientation)
								touchContactsB[idx].OriUpdate = false
							}

							touchContactsB[idx].TUpdate = false
						}
					} else if !contact.Active && contact.TrackUpdate {
						writeEvent(uInputTouch.File, evAbs, absMtSlot, int32(idx))
						writeEvent(uInputTouch.File, evAbs, absMtTrackingId, -1)
						if touchDevice.hasPressure {
							writeEvent(uInputTouch.File, evAbs, absMtPressure, 0)
						}
						if touchDevice.hasOrientation {
							writeEvent(uInputTouch.File, evAbs, absMtOrientation, 0)
						}
						touchContactsB[idx].TrackUpdate = false
						touchContactsB[idx].TUpdate = false
					}
				}

				if activeSlots == 0 && isBtnDown { //Button Up
					isBtnDown = false
					writeEvent(uInputTouch.File, evKey, btnTouch, 0)
				} else if activeSlots > 0 && !isBtnDown { //Button Down
					isBtnDown = true // Button down state change here
					writeEvent(uInputTouch.File, evKey, btnTouch, 1)
				}

				writeEvent(uInputTouch.File, evSyn, synReport, 0)
			}
		default:
		}
	}
}

func touchInputSetup(mode TypeMode, width, height int32) bool {
	tDevs, err := getInputDevices()
	if err != nil {
		return false
	}

	if len(tDevs) < 1 {
		return false
	}
	return touchInputStart(mode, width, height, tDevs[0])
}

func touchInputStart(mode TypeMode, width, height int32, inDev *InputDevice) bool {
	if !touchStart {
		currMode = mode

		//Init Things
		displayWidth = width
		displayHeight = height

		syncChannel = make(chan bool)
		stopChannel = make(chan bool)

		if mode == TYPEA {
			//Setup TypeA UInput Touch Device
			tsDev, err := newTypeADevSame(inDev)
			if err != nil {
				return false
			}

			touchDevice = inDev
			uInputTouch = tsDev

			//Set Default Values in Touch Contacts Array
			touchContactsA = make([]TouchContactA, touchDevice.Slots)
			for idx := range touchContactsA {
				touchContactsA[idx].PosX = -1
				touchContactsA[idx].PosY = -1
				touchContactsA[idx].Active = false
			}

			//Start Threads
			go eventReaderA()
			go eventDispatcherA()
		} else {
			//Setup TypeB UInput Touch Device
			tsDev, err := newTypeBDevSame(inDev)
			if err != nil {
				return false
			}

			touchDevice = inDev
			uInputTouch = tsDev

			if touchDevice.hasTouchMajor {
				fakeTouchMajor = int32(float32(touchDevice.AbsInfos[absMtTouchMajor].Maximum) * 0.14)
			}
			if touchDevice.hasTouchMinor {
				fakeTouchMinor = int32(float32(touchDevice.AbsInfos[absMtTouchMinor].Maximum) * 0.10)
			}
			if touchDevice.hasWidthMajor {
				fakeWidthMajor = int32(float32(touchDevice.AbsInfos[absMtWidthMajor].Maximum) * 0.14)
			}
			if touchDevice.hasWidthMinor {
				fakeWidthMinor = int32(float32(touchDevice.AbsInfos[absMtWidthMinor].Maximum) * 0.10)
			}
			if touchDevice.hasOrientation {
				fakeOrientation = int32(float32(touchDevice.AbsInfos[absMtOrientation].Maximum) * 0.28)
			}
			if touchDevice.hasPressure {
				fakePressure = int32(float32(touchDevice.AbsInfos[absMtPressure].Maximum) * 0.35)
			}

			//Set Default Values in Touch Contacts Array
			touchContactsB = make([]TouchContactB, touchDevice.Slots)
			for idx := range touchContactsB {
				touchContactsB[idx].TouchMajor = -1
				touchContactsB[idx].TouchMinor = -1
				touchContactsB[idx].WidthMajor = -1
				touchContactsB[idx].WidthMinor = -1
				touchContactsB[idx].Orientation = -1
				touchContactsB[idx].PositionX = -1
				touchContactsB[idx].PositionY = -1
				touchContactsB[idx].TrackingId = -1
				touchContactsB[idx].Pressure = -1

				touchContactsB[idx].Active = false
				touchContactsB[idx].TUpdate = false
				touchContactsB[idx].TMAUpdate = false
				touchContactsB[idx].TMIUpdate = false
				touchContactsB[idx].WMAUpdate = false
				touchContactsB[idx].WMIUpdate = false
				touchContactsB[idx].OriUpdate = false
				touchContactsB[idx].PosXUpdate = false
				touchContactsB[idx].PosYUpdate = false
				touchContactsB[idx].TrackUpdate = false
				touchContactsB[idx].PressUpdate = false
			}

			//Start Threads
			go eventReaderB()
			go eventDispatcherB()
		}

		touchStart = true
	}
	return true
}

func touchInputStop() {
	if touchStart && touchDevice != nil && uInputTouch != nil {
		stopChannel <- true

		_ = releaseDevice(uInputTouch.File)
		_ = uInputTouch.File.Close()
		_ = touchDevice.Release()

		uInputTouch = nil
		touchDevice = nil

		touchStart = false
	}
}

///----------Fake Touch Input-----------///

func sendTouchMove(x, y int32) {
	if !touchStart {
		return
	}

	if !touchSend {
		touchSend = true
	}

	x = (x * touchDevice.TouchXMax / displayWidth) + touchDevice.TouchXMin
	y = (y * touchDevice.TouchYMax / displayHeight) + touchDevice.TouchYMin

	if currMode == TYPEA {
		touchContactsA[fakeContact].PosX = x
		touchContactsA[fakeContact].PosY = y
		touchContactsA[fakeContact].Active = true
	} else {
		if touchDevice.hasTouchMajor {
			touchContactsB[fakeContact].TouchMajor = fakeTouchMajor
			touchContactsB[fakeContact].TMAUpdate = true
		}
		if touchDevice.hasTouchMinor {
			touchContactsB[fakeContact].TouchMinor = fakeTouchMinor
			touchContactsB[fakeContact].TMIUpdate = true
		}
		if touchDevice.hasWidthMajor {
			touchContactsB[fakeContact].WidthMajor = fakeWidthMajor
			touchContactsB[fakeContact].WMAUpdate = true
		}
		if touchDevice.hasWidthMinor {
			touchContactsB[fakeContact].WidthMinor = fakeWidthMinor
			touchContactsB[fakeContact].WMIUpdate = true
		}
		if touchDevice.hasOrientation {
			touchContactsB[fakeContact].Orientation = fakeOrientation
			touchContactsB[fakeContact].OriUpdate = true
		}
		if touchDevice.hasPressure {
			touchContactsB[fakeContact].Pressure = fakePressure
			touchContactsB[fakeContact].PressUpdate = true
		}
		if touchContactsB[fakeContact].TrackingId < 0 {
			touchContactsB[fakeContact].TrackingId = touchDevice.AbsInfos[absMtTrackingId].Maximum - 2
			touchContactsB[fakeContact].TrackUpdate = true
		}

		touchContactsB[fakeContact].PositionX = x
		touchContactsB[fakeContact].PositionY = y
		touchContactsB[fakeContact].PosXUpdate = true
		touchContactsB[fakeContact].PosYUpdate = true

		touchContactsB[fakeContact].Active = true
		touchContactsB[fakeContact].TUpdate = true
	}

	syncChannel <- true

	time.Sleep(15 * time.Millisecond)
}

func sendTouchUp() {
	if !touchStart || !touchSend {
		return
	}

	touchSend = false

	if currMode == TYPEA {
		touchContactsA[fakeContact].PosX = -1
		touchContactsA[fakeContact].PosY = -1
		touchContactsA[fakeContact].Active = false
	} else {
		if touchDevice.hasTouchMajor {
			touchContactsB[fakeContact].TouchMajor = -1
		}
		if touchDevice.hasTouchMinor {
			touchContactsB[fakeContact].TouchMinor = -1
		}
		if touchDevice.hasWidthMajor {
			touchContactsB[fakeContact].WidthMajor = -1
		}
		if touchDevice.hasWidthMinor {
			touchContactsB[fakeContact].WidthMinor = -1
		}
		if touchDevice.hasOrientation {
			touchContactsB[fakeContact].Orientation = 0
			touchContactsB[fakeContact].OriUpdate = true
		}
		if touchDevice.hasPressure {
			touchContactsB[fakeContact].Pressure = 0
			touchContactsB[fakeContact].PressUpdate = true
		}

		touchContactsB[fakeContact].TrackingId = -1
		touchContactsB[fakeContact].PositionX = -1
		touchContactsB[fakeContact].PositionY = -1
		touchContactsB[fakeContact].Active = false
		touchContactsB[fakeContact].TUpdate = true
		touchContactsB[fakeContact].TrackUpdate = true
	}

	syncChannel <- true

	time.Sleep(15 * time.Millisecond)
}
