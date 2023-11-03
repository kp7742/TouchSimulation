package main

import (
	"math/rand"
)

const (
	letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func randStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}

	return string(b)
}

func randIntegerNum(n int) int {
	return rand.Intn(n)
}

func randUInt16Num(n int) uint16 {
	return uint16(rand.Intn(n))
}

func getRandomShift() int32 {
	//rand(max - min) - min, Range: [min, max]
	return rand.Int31n(40) - 20 //[-20, 20]
}
