package qr2

import (
	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

// packet sent out to players about the state of the room
type MatchPacket struct {
	Magic                uint32   // magic, always 0x77846772 ("MTCH")
	AidBitmap            uint32   // available aid bitmap
	NumAids              uint32   // num non-guest players
	DirectAidBitmap      uint32   // aid bitmap, including guests.
	RoomID               uint32   // id for the room
	PlayerAid            uint8    // receiving player's aid, will need to set this indivudually
	HostAid              uint8    // aid of the host
	Suspended            bool     // match making suspension state
	RoomCanceled         bool     // whether room is canceled
	AidLocalPlayerCounts []uint32 // local player counts for each player. Size is always 12, even if there isn't 12 players in the room
}

func sendToAid(aidBitmap uint32, numAids uint32, directAidBitmap uint32, roomId uint32, hostAid uint8, roomSuspended bool, roomCanceled bool, localPlayerCounts *[12]uint32, playerAid uint8, playerConnectionIndex uint64) error {
	if playerAid > 11 {
		logging.Error(moduleName, "Invalid player aid (", playerAid, ") when sending match packet")
		return nil
	}

	if hostAid > 11 {
		logging.Error(moduleName, "Invalid host aid (", hostAid, ") when sending match packet")
		return nil
	}

	return common.SendPacket(ServerName, playerConnectionIndex, toByteSlice(aidBitmap, numAids, directAidBitmap, roomId, hostAid, roomSuspended, roomCanceled, localPlayerCounts, playerAid))
}

func (m *MatchPacket) removeAid(aid uint8) {
	if aid > 11 {
		logging.Error(moduleName, "Invalid aid", aurora.Yellow(aid), "when removing aid from match packet")
		return
	}

	m.AidBitmap = common.ClearAid(m.AidBitmap, aid)
	m.DirectAidBitmap = common.ClearAid(m.DirectAidBitmap, aid)
	m.NumAids--
	m.AidLocalPlayerCounts[aid] = 0
}

// pass in the receiver's aid, this allows us to set the aid for each send
func toByteSlice(aidBitmap uint32, numAids uint32, directAidBitmap uint32, roomID uint32, hostAid uint8, roomSuspended bool, roomCanceled bool, localPlayerCounts *[12]uint32, playerAid uint8) []byte {
	pb := &common.PacketBuilder{Buf: make([]byte, 0, 48)}

	pb.WriteUint32(0x77846772)
	pb.WriteUint32(aidBitmap)
	pb.WriteUint32(numAids)
	pb.WriteUint32(directAidBitmap)
	pb.WriteUint32(roomID)
	pb.WriteUint8(playerAid)
	pb.WriteUint8(hostAid)
	pb.WriteBool(roomSuspended)
	pb.WriteBool(roomCanceled)
	for _, count := range *localPlayerCounts {
		pb.WriteUint32(count)
	}

	return pb.Buf
}
