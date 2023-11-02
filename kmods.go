package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

const (
	x  = 746
	y  = 1064
	nx = 400
	ny = 1408
)

func genMovePoints(StartX, StartY, EndX, EndY int32) {
	var minPointCount int32 = 2
	var maxMoveDistance int32 = 10

	dX := float32(EndX) - float32(StartX)
	dY := float32(EndY) - float32(StartY)

	xCount := i32Abs(int32(dX) / maxMoveDistance)
	yCount := i32Abs(int32(dY) / maxMoveDistance)
	count := i32Max(xCount, yCount)
	count = i32Max(count, minPointCount)

	x := float32(StartX)
	y := float32(StartY)
	actDeltaX := dX / float32(count)
	actDeltaY := dY / float32(count)

	for i := 0; i < int(count); i++ {
		sendTouchMove(int32(x+actDeltaX*float32(i)), int32(y+actDeltaY*float32(i)))
	}
}

func i32Abs(i int32) int32 {
	if i < 0 {
		return -i
	}
	return i
}

func i32Max(a int32, b int32) int32 {
	if a < b {
		return b
	}
	return a
}

func Swipe(StartX, StartY, EndX, EndY int32) {
	sendTouchMove(StartX, StartY)

	genMovePoints(StartX, StartY, EndX, EndY)

	sendTouchMove(EndX, EndY)

	sendTouchUp()
}

func main() {
	//Using Common Display Resolution, 2340x1080
	if !touchInputSetup(TYPEB, 1440, 3216) {
		log.Fatalln("No Touch Device Found!")
		return
	}

	time.Sleep(time.Second * 3)

	Swipe(x, y, x, ny)

	time.Sleep(time.Second * 3)

	Swipe(nx, y, x, ny)

	time.Sleep(time.Second * 3)

	Swipe(x, ny, x, y)

	time.Sleep(time.Second * 3)

	Swipe(x, ny, nx, y)

	for {
		reader := bufio.NewReader(os.Stdin)
		exit_code, _ := reader.ReadString('\n')
		fmt.Print(exit_code)
		if strings.Compare(strings.ToLower(exit_code[:len(exit_code)-1]), "exit") == 0 {
			touchInputStop()
			break
		}
		if syncChannel == nil {
			break
		}
	}
}
