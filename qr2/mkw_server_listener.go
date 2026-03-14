package qr2

import (
	"encoding/binary"
	"net"

	"wwfc/common"
	"wwfc/logging"
)

// MKWServerMessageListener listens for messages from MKW-Server servers
func acceptMKWServerMessages() {
	for {
		conn, err := mkwServerListener.Accept()
		if err != nil {
			logging.Error("MKW-Server Manager", "Error accepting connection:", err)
			continue
		}
		go readMKWServerMessage(conn)
	}
}

func readMKWServerMessage(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 128)
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			logging.Error("MKW-Server Manager", "Error reading from connection:", err)
			return
		}
		logging.Info("MKW-Server Manager", "Read", n, "bytes from MKW-Server. buffer:", string(buffer))
		msg := buffer[:n]
		handleMessageFromMKWServer(conn, msg)
	}
}

func handleMessageFromMKWServer(conn net.Conn, msg []byte) {
	if len(msg) == 0 {
		return
	}

	logging.Info("MKW-Server Manager", "Received message:", msg)

	var matchRequest MatchRequestType = MatchRequestType(msg[0])
	switch matchRequest {
	case OpenFroom:
		logging.Info(moduleName, "handling OpenRoom")
		mkwServer := getMKWServerByPort(msg)
		if mkwServer == nil {
			logging.Info(moduleName, "Couldn't find mkwServer!")
			return
		}

		// were done once we set the room address and connection
		mkwServer.conn = conn

		room := mkwServer.roomPointer
		if room == nil {
			logging.Info(moduleName, "mkwServer.roomPointer is nil")
			return
		}

		host := room.host
		if host == nil {
			logging.Info(moduleName, "room has no host (public room)")
			return
		}

		// mkw-server is good to add players and communicate with them, send an AddPlayer request
		mkwServer.sendJoinFroom(host)

	case JoinFroom:
		logging.Info(moduleName, "handling AddPlayer")
		if len(msg) != 9 {
			logging.Info(moduleName, "Invalid PlayerAdded msg len:", len(msg))
			return
		}

		searchId := binary.BigEndian.Uint64(msg[1:])
		logging.Info(moduleName, "AddPlayer searchId", searchId)

		player := playerBySearchID[searchId]
		if player == nil {
			logging.Info(moduleName, "Couldn't find player by search id! SearchId:", searchId)
			return
		}

		// get the room's mkwServer
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

		// send mkw-server's address to the player
		common.SendPacket(ServerName, player.roomManagerConnnectionIndex, MakeMKWServerAddressPacket(mkwServer.udpAddr))

	case LeaveFroom:
		logging.Info("MKW-Server Manager", "Handling CloseRoom")

		mkwServer := getMKWServerByPort(msg)
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
