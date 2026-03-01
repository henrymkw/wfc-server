package common

func aidSlot(aid uint8) uint32 {
	return 1 << aid
}

func SetAid(aids uint32, aid uint8) uint32 {
	return aids | aidSlot(aid)
}

func ClearAid(aids uint32, aid uint8) uint32 {
	return aids & ^aidSlot(aid)
}

// get the next available aid. pass in the aidBitmap and loop over (at most) 12 aid slot bits, and find the lsb thats 0
func GetAvailableAid(aidBitmap uint32) uint8 { // TODO: Change to error type or something
	var i uint8
	for i = range 12 {
		if ((aidBitmap >> i) & 1) == 0 {
			return i
		}
		// otherwise continue
	}

	return 0xff
}

func SetLocalPlayerCount(localPlayerCount uint32) uint32 {
	return localPlayerCount << 24
}

func GetAidPlayerCount(playerCount uint32) uint32 {
	return playerCount >> 24
}
