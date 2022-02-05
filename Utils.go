package main

import (
	"math/rand"
	"time"
)

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func randStringBytes(n int) string {
	rand.Seed(time.Now().UnixNano())

	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}

	return string(b)
}

func randIntegerNum(n int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(n)
}

func randUInt16Num(n int) uint16 {
	rand.Seed(time.Now().UnixNano())
	return uint16(rand.Intn(n))
}

func getRandomShift() int32 {
	rand.Seed(time.Now().UnixNano())
	//rand(max - min) - min, Range: [min, max]
	return rand.Int31n(40) - 20 //[-20, 20]
}
