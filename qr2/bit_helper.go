package qr2

import (
	"errors"
	"math/bits"
)

func aidSlot(aid uint8) uint32 {
	return 1 << aid
}

func setAid(aids uint32, aid uint8) uint32 {
	return aids | aidSlot(aid)
}

func clearAid(aids uint32, aid uint8) uint32 {
	return aids & ^aidSlot(aid)
}

func getAvailableAid(aidBitmap uint32) (uint8, error) {
	// Flip all bits, the first 1 found is the available aid
	aid := bits.TrailingZeros32(^aidBitmap)
	if aid >= MaxPlayerCount {
		return 0xff, errors.New("No available aid!")
	}
	return uint8(aid), nil
}
