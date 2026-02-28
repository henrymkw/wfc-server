package common

func aidSlot(aid uint8) uint32 {
	return 1 << aid
}

func SetAid(aids uint32, aid uint8) uint32 {
	return aids | aidSlot(aid)
}

func ClearAid(aids uint32, aid uint8) uint32 {
	return aids &^ aidSlot(aid)
}

func SetAidPlayerCount(has2Players bool) uint32 {
	if has2Players {
		return 1 << 24
	} else {
		return 0
	}
}

func GetAidPlayerCount(playerCount uint32) uint32 {
	return playerCount >> 24
}
