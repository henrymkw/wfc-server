package qr2

import (
	"net"

	"os/exec"
	"wwfc/common"
	"wwfc/logging"
)

const moduleName = "QR2"

func createMKWServer(r *Room) *MKWServer {
	// localhost if mkw-server is to spawn on the same machine, more logic would need to be added for remote mkw-server servers

	// for now just use the gamespy address
	mkwServerIP := net.ParseIP(*common.GetConfig().GameSpyAddress)

	// use a random port to be less predictable and also avoid conflicts
	mkwServerPort, err := findOpenUDPPort(26000, 26999)
	if err != nil {
		logging.Error(moduleName, "Failed to find open UDP port:", err)
		return nil
	}

	mkwServerAddr := net.UDPAddr{
		IP:		mkwServerIP,
		Port:	mkwServerPort,
	}

	mkwServerPath := common.GetConfig().MkwServerPath

	logging.Info(moduleName, "Using mkw-server executable at", mkwServerPath)
	cmd := exec.Command(
		mkwServerPath,
		"--room-addr", mkwServerAddr.String(),
		"--wfc-addr", mkwServerListener.Addr().String(),
	)

	logging.Info(moduleName, "mkw-server cmd", cmd)

	if err := cmd.Start(); err != nil {
		logging.Error(moduleName, "Failed to start mkw-server process:", err)
		return nil
	}

	go func(c *exec.Cmd) {
		err := c.Wait()
		if err != nil {
			logging.Error(moduleName, "mkw-server process exited with error:", err)

		} else {
			logging.Info(moduleName, "mkw-server process exited successfully")
		}
	}(cmd)

	mkwServer := &MKWServer{
		Cmd:            cmd,
		isRemote:       false,
		udpAddr:        mkwServerAddr, // set later by ROOM_OPEN message
		conn: 			nil, // set later by ROOM_OPEN message
		roomPointer:    r,
	}

	mkwServers[mkwServerPort] = mkwServer

	logging.Info(moduleName, "Created mkwServer for room", r.roomName, "at", mkwServerAddr.String())

	return mkwServer
}

// this will tell mkw-server to update its state since a player joined
func (mkwServer *MKWServer) sendAddPlayerRequest(player *Player) {
	if player == nil {
		logging.Info(moduleName, "sendAddPlayerRequest player is nil")
		return
	}

	// pack up data into a packet
	ip, port := common.IPFormatToInt(player.Addr.String())

	newPlayer := NewPlayerMessage{
		matchRequest: 	JoinFroom,
		ip: 			ip,
		port: 			port,
		aid: 			player.aid,
		isHost: 		player.isHost,
		searchId: 		player.SearchId,
	}
	
	logging.Info(moduleName, "sendAddPlayerRequest player.searchId", player.SearchId)

	// send it to mkw-server
	mkwServer.conn.Write(newPlayer.toBytes())
}

func (mkwServer *MKWServer) sendMkwServerRemoveClient(player *Player) {
	if mkwServer.conn == nil {
		logging.Error(moduleName, "MkwServerInfo.WfcMkwServerConn is nil. Cannot send remove client message")
		return
	}

	if player == nil {
		logging.Error(moduleName, "player is nil. Cannot send remove client message")
		return
	}

	// send over {0x3, 0x0, 0x0, 0x0} followed by the addr string
	message := make([]byte, 0)
	message = append(message, ServerLeaveFroomRequest)
	message = append(message, 0x0)
	message = append(message, 0x0)
	message = append(message, 0x0)
	message = append(message, player.Addr.String()...)

	mkwServer.conn.Write(message)
}
