package qr2

import (
	"bytes"
	"encoding/binary"
	"net"

	"wwfc/common"
)

const MKWServerAddressPacketMagic uint32 = 0x4D4B5753 // 'MKWS'

// packet sent to mkw-server informing it about a new player joining
type NewPlayerMessage struct {
	matchRequest MatchRequestType // Always should be JoinFroom (1)
	ip           int32            // players ip
	port         uint16           // players port
	aid          uint8            // player's aid
	isHost       bool             // whether the player is host
	searchId     uint64           // might not be needed?
}

func (msg *NewPlayerMessage) toBytes() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint8(msg.matchRequest))
	binary.Write(buf, binary.BigEndian, msg.ip)
	binary.Write(buf, binary.BigEndian, msg.port)
	binary.Write(buf, binary.BigEndian, msg.aid)
	binary.Write(buf, binary.BigEndian, msg.isHost)
	binary.Write(buf, binary.BigEndian, msg.searchId)
	return buf.Bytes()
}

func MakeMKWServerAddressPacket(addr net.UDPAddr) []byte {
	ip, port := common.IPFormatToInt(addr.String())
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, MKWServerAddressPacketMagic)
	binary.Write(buf, binary.BigEndian, ip)
	binary.Write(buf, binary.BigEndian, port)
	return buf.Bytes()
}
