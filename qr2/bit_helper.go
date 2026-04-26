package qr2

import (
	"errors"
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

// get the next available aid. pass in the aidBitmap and loop over (at most) 12 aid slot bits, and find the lsb thats 0
func getAvailableAid(aidBitmap uint32) (uint8, error) { // TODO: Change to error type or something
	var i uint8
	for i = range 12 {
		if ((aidBitmap >> i) & 1) == 0 {
			return i, nil
		}
		// otherwise continue
	}

	return 0xff, errors.New("No available aid!")
}

func setLocalPlayerCount(localPlayerCount uint32) uint32 {
	return localPlayerCount << 24
}

func getAidPlayerCount(playerCount uint32) uint32 {
	return playerCount >> 24
}
