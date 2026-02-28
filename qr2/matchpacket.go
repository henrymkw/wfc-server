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

// called when room is created
func initMatchPacket(roomID uint32, isHostLocal2Players bool) *MatchPacket {
	// TODO: set necessary fields for player and room

	aidPlayerCounts := make([]uint32, 12)
	aidPlayerCounts[0] = common.SetAidPlayerCount(isHostLocal2Players)

	return &MatchPacket{
		Magic:                0x77846772,
		AidBitmap:            common.SetAid(0, 0),
		NumAids:              1,
		DirectAidBitmap:      common.SetAid(0, 0),
		RoomID:               roomID,
		PlayerAid:            0,
		HostAid:              0,
		Suspended:            false,
		RoomCanceled:         false,
		AidLocalPlayerCounts: aidPlayerCounts,
	}
}

func (m *MatchPacket) sendToAid(aid uint8, connectionIndex uint64) error {
	if aid > 11 {
		logging.Error(moduleName, "Invalid aid", aurora.Yellow(aid), "when sending match packet")
		return nil
	}

	return common.SendPacket(ServerName, connectionIndex, m.toByteSlice(aid))
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
func (m *MatchPacket) toByteSlice(aid uint8) []byte {
	pb := &common.PacketBuilder{Buf: make([]byte, 0, 48)}

	pb.WriteUint32(m.Magic)
	pb.WriteUint32(m.AidBitmap)
	pb.WriteUint32(m.NumAids)
	pb.WriteUint32(m.DirectAidBitmap)
	pb.WriteUint32(m.RoomID)
	pb.WriteUint8(aid)
	pb.WriteUint8(m.HostAid)
	pb.WriteBool(m.Suspended)
	pb.WriteBool(m.RoomCanceled)
	for _, count := range m.AidLocalPlayerCounts {
		pb.WriteUint32(count)
	}

	return pb.Buf
}
