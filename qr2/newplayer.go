package qr2

import (
	"net"

	"wwfc/common"
)

// packet sent to mkw-server informing it about a new player joining a froom
type NewPlayerMessage struct {
	matchRequest 	MatchRequestType // Always should be JoinFroom (1)
	ip 				int32 // players ip
	port			uint16 // players port
	aid				uint8 // player's aid
	isHost			bool // whether the player is host
	searchId		uint64 // might not be needed?
}

func (msg *NewPlayerMessage) toBytes() []byte {
	pb := &common.PacketBuilder{Buf: make([]byte, 0, 17)}

	pb.WriteUint8(uint8(msg.matchRequest))
	pb.WriteInt32(msg.ip)
	pb.WriteUint16(msg.port)
	pb.WriteUint8(msg.aid)
	pb.WriteBool(msg.isHost)
	pb.WriteUint64(msg.searchId)

	return pb.Buf
}

// Packet sent to players containing the address of mkw-server
// its format is a 4 byte magic, 4 byte ip, 2 byte port, and 2 bytes of padding
func MakeMKWServerAddressPacket(addr net.UDPAddr) []byte {
	ip, port := common.IPFormatToInt(addr.String())

	pb := &common.PacketBuilder{Buf: make([]byte, 0, )}
	pb.WriteUint32(0x4D4B5753) // 'MKWS', magic
	pb.WriteInt32(ip)
	pb.WriteUint16(port)

	return pb.Buf
}
