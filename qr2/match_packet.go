package qr2

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"wwfc/common"
)

const MatchMakingPacketMagic uint32 = 0x77846772 // 'MTCH'

func sendToAid(aidBitmap uint32, numAids uint32, directAidBitmap uint32, roomId uint32, hostAid uint8, roomSuspended bool, roomCanceled bool, localPlayerCounts *[MaxPlayerCount]uint32, playerAid uint8, playerConnectionIndex uint64) error {
	if playerAid > MaxAid && playerAid != NoAid {
		return fmt.Errorf("Invalid player aid (%d) when sending match packet", playerAid)
	}

	if hostAid > MaxAid && hostAid != NoAid {
		return fmt.Errorf("Invalid host aid (%d) when sending match packet", hostAid)
	}

	return common.SendPacket(ServerName, playerConnectionIndex, toByteSlice(aidBitmap, numAids, directAidBitmap, roomId, hostAid, roomSuspended, roomCanceled, localPlayerCounts, playerAid))
}

// pass in the receiver's aid, this allows us to set the aid for each send
func toByteSlice(aidBitmap uint32, numAids uint32, directAidBitmap uint32, roomID uint32, hostAid uint8, roomSuspended bool, roomCanceled bool, localPlayerCounts *[MaxPlayerCount]uint32, playerAid uint8) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, MatchMakingPacketMagic)
	binary.Write(buf, binary.BigEndian, aidBitmap)
	binary.Write(buf, binary.BigEndian, numAids)
	binary.Write(buf, binary.BigEndian, directAidBitmap)
	binary.Write(buf, binary.BigEndian, roomID)
	binary.Write(buf, binary.BigEndian, playerAid)
	binary.Write(buf, binary.BigEndian, hostAid)
	binary.Write(buf, binary.BigEndian, roomSuspended)
	binary.Write(buf, binary.BigEndian, roomCanceled)
	for _, count := range *localPlayerCounts {
		binary.Write(buf, binary.BigEndian, count)
	}
	return buf.Bytes()
}
