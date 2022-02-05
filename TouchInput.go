package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/lunixbochs/struc"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	fakeContact = 9
)

var (
	isBtnDown  = false
	touchSend  = false
	touchStart = false

	touchXMin     int32
	touchXMax     int32
	touchYMin     int32
	touchYMax     int32
	displayWidth  int32
	displayHeight int32

	syncChannel chan bool
	stopChannel chan bool

	touchDevice *InputDevice
	uInputTouch *InputDevice

	inputDevices  []*InputDevice
	touchContacts []TouchContact
)

///----------Touch Contacts-----------///

// TouchContact Touch Contact Struct
type TouchContact struct {
	PosX   int32
	PosY   int32
	Active bool
}

///----------Touch Management Interface-----------///

//Read Input Event from Input Device
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

//Write Input Event to Specified Fd
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

//Reading Touch Inputs
func eventReader() {
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
				touchContacts[currSlot].Active = inputEvent.Value != -1
				fmt.Printf("ABS_MT_TRACKING_ID: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtPositionX:
				touchContacts[currSlot].PosX = inputEvent.Value
				fmt.Printf("ABS_MT_POSITION_X: %d | Slot: %d\n", inputEvent.Value, currSlot)
				break
			case absMtPositionY:
				touchContacts[currSlot].PosY = inputEvent.Value
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

//Writing Touch Inputs
func eventDispatcher() {
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

				for idx, contact := range touchContacts {
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

func touchInputSetup(width, height int32) {
	if inputDevices == nil {
		tDevs, err := getInputDevices()
		if err != nil {
			return
		}
		inputDevices = tDevs
	}

	touchInputStart(width, height, inputDevices[0])
}

func touchInputStart(width, height int32, inDev *InputDevice) {
	if !touchStart {
		//Setup UInput Touch Device
		tsDev, err := newUInputDevice(inDev)
		if err != nil {
			return
		}

		touchDevice = inDev
		uInputTouch = tsDev

		//Init Things
		displayWidth = width
		displayHeight = height

		syncChannel = make(chan bool)
		stopChannel = make(chan bool)

		touchXMin = touchDevice.AbsX.Minimum
		touchXMax = touchDevice.AbsX.Maximum - touchDevice.AbsX.Minimum + 1
		touchYMin = touchDevice.AbsY.Minimum
		touchYMax = touchDevice.AbsY.Maximum - touchDevice.AbsY.Minimum + 1

		//Set Default Values in Touch Contacts Array
		touchContacts = make([]TouchContact, touchDevice.Slot.Maximum+1)
		for idx := range touchContacts {
			touchContacts[idx].PosX = -1
			touchContacts[idx].PosY = -1
			touchContacts[idx].Active = false
		}

		//Start Threads
		go eventReader()
		go eventDispatcher()

		touchStart = true
	}
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

	touchContacts[fakeContact].PosX = (x * touchXMax / displayWidth) + touchXMin
	touchContacts[fakeContact].PosY = (y * touchYMax / displayHeight) + touchYMin
	touchContacts[fakeContact].Active = true

	syncChannel <- true

	time.Sleep(15 * time.Millisecond)
}

func sendTouchUp() {
	if !touchStart || !touchSend {
		return
	}

	touchSend = false

	touchContacts[fakeContact].PosX = -1
	touchContacts[fakeContact].PosY = -1
	touchContacts[fakeContact].Active = false

	syncChannel <- true

	time.Sleep(15 * time.Millisecond)
}
