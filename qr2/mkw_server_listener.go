package qr2

import (
	"encoding/binary"
	"net"

	"wwfc/common"
	"wwfc/logging"
)

// In TCP, messages can get cut off in a single send and be incomplete. But they can be
// completed in subsequent receives from mkw-server. We store each mkw-servers messages
// in a buffer to be able to complete them later
var mkwServerMessageBuffer map[string]*[]byte

// key is conn.RemoteAddr().String(), which is the only unique identifier in a net.Conn
// This is fine if wfc-server and mkw-server are on the same machine and listening to messages on localhost.
// But this is very dangerous for remote mkw-servers, since the address could be spoofed.
// So this map structure must be changed when remote mkw-server is implemented

// MKWServerMessageListener listens for messages from MKW-Server servers
func acceptMKWServerMessages() {
	mkwServerMessageBuffer = make(map[string]*[]byte)
	for {
		conn, err := mkwServerListener.Accept()
		if err != nil {
			logging.Error(moduleName, "Error accepting connection:", err)
			continue
		}
		go readMKWServerMessage(conn)
	}
}

func readMKWServerMessage(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 256)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			logging.Error(moduleName, "Error reading from connection:", err)
			return
		}
		msg := buffer[:n]
		handleMessageFromMKWServer(conn, msg)
	}
}

func handleMessageFromMKWServer(conn net.Conn, msg []byte) {
	if len(msg) == 0 {
		return
	}

	addr := conn.RemoteAddr().String()
	buffer := addMsgToBuffer(addr, msg)
	if buffer == nil {
		logging.Info(moduleName, addr, "returned nil buffer")
		return
	}

	for {
		// note that retreiveCompleteMessage() updates the buffer if there's a complete message
		completeMessage := retreiveCompleteMessageUpdateBuf(buffer)
		if completeMessage == nil {
			break
		}
		if len(completeMessage) == 0 {
			continue
		}
		handleCompleteMessage(completeMessage, conn)
	}
}

func handleCompleteMessage(completeMessage []byte, conn net.Conn) {
	var matchRequest MatchRequestType = MatchRequestType(completeMessage[0])
	switch matchRequest {
	case MKWServerLog:
		logging.Notice("MKW-Server Log", string(completeMessage[1:]))

	case OpenFroom:
		logging.Info(moduleName, "Handling OpenRoom")
		mkwServer := getMKWServerByPort(completeMessage)
		if mkwServer == nil {
			logging.Info(moduleName, "Couldn't find mkwServer!")
			return
		}
		room := mkwServer.roomPointer
		if room == nil {
			logging.Info(moduleName, "mkwServer.roomPointer is nil")
			return
		}
		if room.mkwServer == nil {
			logging.Info(moduleName, "Room", room.roomID, "mkwServer is nil")
			return
		}
		room.mkwServer.conn = conn
		err := room.sendMKWServerJoinRoomForEachPlayer()
		if err != nil {
			logging.Info(moduleName, err)
			return
		}
		logging.Info(moduleName, "Handled OpenFroom!")

	case JoinFroom:
		logging.Info(moduleName, "Handling AddPlayer")
		if len(completeMessage) != 9 {
			logging.Info(moduleName, "Invalid PlayerAdded msg len:", len(completeMessage))
			return
		}
		searchId := binary.BigEndian.Uint64(completeMessage[1:])
		logging.Info(moduleName, "AddPlayer searchId", searchId)
		player := playerBySearchID[searchId]
		if player == nil {
			logging.Info(moduleName, "Couldn't find player by search id! SearchId:", searchId)
			return
		}
		room := player.roomPointer
		if room == nil {
			logging.Info(moduleName, "Player isn't in a room!")
			return
		}
		mkwServer := room.mkwServer
		if mkwServer == nil {
			logging.Info(moduleName, "Room's mkwServer is nil!")
			return
		}
		common.SendPacket(ServerName, player.roomManagerConnnectionIndex, MakeMKWServerAddressPacket(mkwServer.udpAddr))

	case LeaveFroom:
		logging.Info("MKW-Server Manager", "Handling CloseRoom")
		mkwServer := getMKWServerByPort(completeMessage) // was using completedMessage before
		if mkwServer == nil {
			logging.Info(moduleName, "Couldn't find mkwServer!")
			return
		}
		if mkwServer.conn != nil {
			mkwServer.conn.Close()
			mkwServer.conn = nil
			logging.Info(moduleName, "closed mkwServer connection with wfc-server")
		} else {
			logging.Info("MKW-Server Manager", "connToMKWServer already nil")
		}

	default:
		logging.Info(moduleName, "invalid matchRequest:", matchRequest)
	}
}
