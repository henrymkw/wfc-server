package qr2

import (
	"fmt"
	"math/rand"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"wwfc/logging"
)

type MKWServerProxy struct {
	// process of the mkw server. i think this wouldnt work for remove servers
	Cmd             *exec.Cmd
	isRemote        bool         // TODO: Unused for now
	roomAddr        *net.UDPAddr // the udp address of the mkw server (clients send/receive here)
	connToMKWServer net.Conn     // connection to the mkw server's WFC listener
	roomPointer    *Room
}

// key is the room address, easy for clients/rooms to lookup
var mkwServerProxies = map[string]*MKWServerProxy{}

const (
	// Requests sent from the client
	ServerOpenFroomRequest  = 0x01
	ServerJoinFroomRequest  = 0x02
	ServerLeaveFroomRequest = 0x03
)

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
