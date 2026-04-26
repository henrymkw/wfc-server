package qr2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"wwfc/logging"
)

type MKWServer struct {
	// process of the mkw server. i think this wouldnt work for remove servers
	cmd         *exec.Cmd
	isRemote    bool        // TODO: Unused for now
	udpAddr     net.UDPAddr // the udp address of the mkw server (clients send/receive here)
	conn        net.Conn    // connection to the mkw server's WFC listener
	roomPointer *Room
}

// key is the room address, easy for clients/rooms to lookup
var mkwServers = map[int]*MKWServer{}

const (
	// Requests sent from the client
	ServerOpenFroomRequest  = 0x01
	ServerJoinFroomRequest  = 0x02
	ServerLeaveFroomRequest = 0x03
)

func getMKWServerByPort(msg []byte) (*MKWServer, error) {
	if len(msg) != 3 {
		return nil, fmt.Errorf("Invalid msg len to get mkwServer by port:", len(msg))
	}

	// This is safe since we just checked the total size is 3
	port := int(binary.BigEndian.Uint16(msg[1:]))

	mkwServer := mkwServers[port]
	if mkwServer == nil {
		return nil, fmt.Errorf("no mkwServer at port", port)
	}
	logging.Info(moduleName, "found mkwServer at port:", port)
	return mkwServer, nil
}

func findOpenUDPPort(low, high int) (int, error) {
	if low > high {
		return 0, fmt.Errorf("invalid range")
	}

	total := high - low + 1
	// random start point
	start := rand.Intn(total)

	for i := 0; i < total; i++ {
		port := low + ((start + i) % total)
		addr := fmt.Sprintf(":%d", port)
		conn, err := net.ListenPacket("udp", addr)
		if err == nil {
			conn.Close()
			return port, nil
		}
	}

	return 0, fmt.Errorf("no open UDP ports in range %d-%d", low, high)
}

func convertAddrToUDPAddr(address string) (*net.UDPAddr, error) {
	// set the roomAddr
	roomAddrParts := strings.Split(address, ":")
	if len(roomAddrParts) != 2 {
		logging.Error("MKW-Server Manager", "Invalid ROOM_OPEN format:", address)
		return nil, fmt.Errorf("invalid ROOM_OPEN format")
	}
	ip := roomAddrParts[0]
	port := roomAddrParts[1]
	udpPort, err := strconv.Atoi(port)
	if err != nil {
		logging.Error("MKW-Server Manager", "Invalid port in ROOM_OPEN:", port)
		return nil, fmt.Errorf("invalid port in ROOM_OPEN")
	}
	return &net.UDPAddr{
		IP:   net.ParseIP(ip),
		Port: udpPort,
	}, nil
}

func convIPToBytes(ip string) ([]byte, error) {
	ipParts := strings.Split(ip, ".")
	if len(ipParts) != 4 {
		return nil, fmt.Errorf("invalid IP format")
	}
	ipBytes := make([]byte, 4)
	for i, part := range ipParts {
		p, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid IP part: %s", part)
		}
		ipBytes[i] = byte(p)
	}
	return ipBytes, nil
}

func prefixIdx(buf []byte) int {
	return bytes.Index(buf, []byte{0xbb, 0xef, 0xdc, 0xc8})
}

// buffer must start with 0xbb, 0xef, 0xdc, 0xc8 or be empty
func verifyPrefix(buf []byte) bool {
	if len(buf) == 0 || (len(buf) >= 4 && prefixIdx(buf) == 0) {
		return true
	}

	logging.Info(moduleName, "Prefix is invalid due to nil buffer")
	return false
}

func addMsgToBuffer(addr string, msg []byte) *[]byte {
	buffer := mkwServerMessageBuffer[addr]
	if buffer == nil {
		buffer = &[]byte{}
		mkwServerMessageBuffer[addr] = buffer
	}
	if len(*buffer)+len(msg) > 0x500 {
		logging.Error(moduleName, addr, "sent a message that would overflow the buffer!")
		delete(mkwServerMessageBuffer, addr)
		return nil
	}
	combined := append(*buffer, msg...)
	if !verifyPrefix(combined) {
		logging.Error(moduleName, addr, "sent an invalid prefix!")
		delete(mkwServerMessageBuffer, addr)
		return nil
	}
	*buffer = combined
	return buffer
}

// a message is complete if it contains the prefix and suffix defined in the function
func retreiveCompleteMessageUpdateBuf(buf *[]byte) []byte {
	bufContents := *buf
	// returns 0 if the message starts with the prefix, needed in part to be complete
	if prefixIdx(bufContents) != 0 {
		return nil
	}
	// its not complete if the suffix can't be found
	suffixIdx := bytes.Index(bufContents, []byte{0xce, 0xf9, 0xd3, 0xaa})
	if suffixIdx == -1 {
		return nil
	}
	// update the buffer to the next message, and return the complete message
	*buf = (*buf)[suffixIdx+4:]
	return bufContents[4:suffixIdx]
}
