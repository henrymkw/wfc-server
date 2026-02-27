package qr2

import (
	"net"
	"strconv"
	"strings"
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
	logging.Info("MKW-Server Manager", "Received message:", string(msg))

	// For now, messages are simple space-separated strings
	parts := strings.Fields(string(msg))

	if len(parts) == 0 {
		logging.Info("MKW-Server Manager", "Empty message received")
		return
	}

	responceType := parts[0]
	switch responceType {
	case "ROOM_OPEN":
		// ROOM_OPEN doesn't send back to clients, it just sets the udpAddr and connToMKWServer in mkwServerProxy

		// ROOM_OPEN {roomIp:roomPort}
		roomAddress := parts[1]
		logging.Info("MKW-Server Manager", "Handling ROOM_OPEN on roomAddress", roomAddress)

		if mkwServerProxies[roomAddress] == nil {
			logging.Error("MKW-Server Manager", "No active MKW Server info found for room address:", roomAddress)
			return
		}

		if mkwServerProxies[roomAddress].roomAddr != nil {
			logging.Info("MKW-Server Manager", "MKWServerProxy already has roomAddr set, skipping")
			return
		}

		udpAddr, err := convertAddrToUDPAddr(roomAddress)
		if err != nil {
			logging.Error("MKW-Server Manager", "Failed to convert ROOM_OPEN address to UDPAddr:", err)
			return
		}

		// were done once we set the room address and connection
		mkwServerProxies[roomAddress].roomAddr = udpAddr
		mkwServerProxies[roomAddress].connToMKWServer = conn

	case "NEW_PLAYER":
		logging.Info("MKW-Server Manager", "Handling NEW_PLAYER for", parts[1])

		if len(parts) != 3 {
			logging.Error("MKW-Server Manager", "Invalid NEW_PLAYER message format:", string(msg))
			return
		}

		// The message we get back from mkw-server is "NEW_PLAYER {clientIp:clientPort} {mkwserverIp:mkwserverPort}"
		// We need to split parts[1] to get the client ip/port so we can send them the mkw-server address
		playerAddr := parts[1]
		player := players[makeLookupAddr(playerAddr)]
		if player == nil {
			logging.Error("MKW-Server Manager", "No player found for player address:", playerAddr)
			return
		}

		if player.roomPointer == nil {
			logging.Error("MKW-Server Manager", "player has no roomPointer for player address:", playerAddr)
			return
		}

		// the mkwServerProxy should have all its fields set by now
		mkwServerProxy := player.roomPointer.mkwServerProxy
		if mkwServerProxy == nil {
			logging.Error("MKW-Server Manager", "No MKWServerProxy found for player's room")
			return
		}

		// parts[2] is the room address which the room sent itself
		// we compare mkwServerProxy.roomAddr to ensure its the same room
		if mkwServerProxy.roomAddr == nil {
			logging.Error("MKW-Server Manager", "MKWServerProxy has nil roomAddr")
			return
		}
		roomAddrStr := mkwServerProxy.roomAddr.String()
		if roomAddrStr != parts[2] {
			logging.Error("MKW-Server Manager", "MKWServerProxy roomAddr does not match NEW_PLAYER room address:", roomAddrStr, "vs", parts[2])
			return
		}
		// Then we need to form the message, which starts with {0xC, 0x1, 0x0, 0x0}
		// followed by the ip (4 bytes hex) and port (2 bytes hex) (0x0, 0x0, port high, port low) of mkw-server

		message := make([]byte, 0)
		message = append(message, 0xC)
		message = append(message, 0x1)
		message = append(message, 0x0)
		message = append(message, 0x0)

		mkwServerAddrSplit := strings.Split(roomAddrStr, ":")
		if len(mkwServerAddrSplit) != 2 {
			logging.Error("MKW-Server Manager", "Invalid NEW_PLAYER mkw-server address format:", parts[2])
			return
		}
		roomIp, err := convIPToBytes(mkwServerAddrSplit[0])
		if err != nil {
			logging.Error("MKW-Server Manager", "Failed to convert MKW-Server IP to bytes:", err)
			return
		}
		message = append(message, roomIp...)

		mkwServerPortNum, err := strconv.Atoi(mkwServerAddrSplit[1])
		if err != nil {
			logging.Error("MKW-Server Manager", "Invalid MKW-Server port number:", mkwServerAddrSplit[1])
			return
		}
		message = append(message, byte((mkwServerPortNum>>8)&0xFF))
		message = append(message, byte(mkwServerPortNum&0xFF))

		logging.Info("MKW-Server Manager", "Constructed message to send to client:", message)

		// send to the client
		masterConn.WriteTo([]byte(message), &player.Addr)
		logging.Info("MKW-Server Manager", "Sent MKW-Server address to client at", player.Addr.String())

	case "MKWSERVER_SHUTDOWN":
		logging.Info("MKW-Server Manager", "Handling MKWServer_SHUTDOWN for", parts[1])

		mkwServerProxy := mkwServerProxies[parts[1]]
		if mkwServerProxy == nil {
			logging.Error("MKW-Server Manager", "No active MKW Server info found for address:", parts[1])
			return
		}

		if mkwServerProxy.connToMKWServer != nil {
			mkwServerProxy.connToMKWServer.Close()
			mkwServerProxy.connToMKWServer = nil
			logging.Info("MKW-Server Manager", "Closed connToMKWServer for address:", parts[1])
		} else {
			logging.Info("MKW-Server Manager", "connToMKWServer already nil for address:", parts[1])
		}

	}
}
