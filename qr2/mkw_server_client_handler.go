package qr2

import (
	"errors"
	"net"
	"os/exec"
	"syscall"

	"wwfc/common"
	"wwfc/logging"
)

const moduleName = "QR2"

func startMKWServer(r *Room) *MKWServer {
	// for now just use the gamespy address. Only works if wfc-server and mkw-server are on the same machine
	mkwServerIP := net.ParseIP(*common.GetConfig().GameSpyAddress)
	if mkwServerIP == nil {
		logging.Error(moduleName, "Failed to parse mkw-server's ip from the config. Please verify it's formatted correctly!")
		return nil
	}

	// use a random port to be less predictable and also avoid conflicts
	mkwServerPort, err := findOpenUDPPort(26000, 26999)
	if err != nil {
		logging.Error(moduleName, "Failed to find open UDP port:", err)
		return nil
	}

	mkwServerAddr := net.UDPAddr{
		IP:   mkwServerIP,
		Port: mkwServerPort,
	}

	mkwServerPath := common.GetConfig().MkwServerPath

	cmd := exec.Command(
		mkwServerPath,
		"--room-addr", mkwServerAddr.String(),
		"--wfc-addr", mkwServerListener.Addr().String(),
	)

	logging.Info(moduleName, "Running command to start mkw-server process:", cmd)

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
		cmd:         cmd,
		isRemote:    false,
		udpAddr:     mkwServerAddr, // set later by ROOM_OPEN message
		conn:        nil,           // set later by ROOM_OPEN message
		roomPointer: r,
	}

	mkwServers[mkwServerPort] = mkwServer

	logging.Info(moduleName, "Created mkw-server for room", r.roomID, "at TCP port", mkwServerPort)

	return mkwServer
}

func (mkwServer *MKWServer) terminateProcess() {
	if mkwServer.cmd == nil && mkwServer.cmd.Process == nil {
		logging.Info(moduleName, "Can't terminate mkw-server process, its already nil (already terminated?)")
		return
	}

	mkwServer.cmd.Process.Signal(syscall.SIGTERM)
	logging.Info(moduleName, "Terminated MKW-Server process!")
}

// this will tell mkw-server to update its state since a player joined
func (mkwServer *MKWServer) sendJoinRoom(player *Player) error {
	if player == nil {
		logging.Info(moduleName, "sendAddPlayerRequest player is nil")
		return errors.New("player passed into sendJoinRoom is nil!")
	}

	// pack up data into a packet
	ip, port := common.IPFormatToInt(player.Addr.String())

	newPlayer := NewPlayerMessage{
		matchRequest: JoinFroom,
		ip:           ip,
		port:         port,
		aid:          player.aid,
		isHost:       player.isHost,
		searchId:     player.SearchId,
	}

	// send it to mkw-server
	if mkwServer.conn == nil {
		return errors.New("The connection to mkw-server is nil! This is bad and shouldn't happen just before sending.")
	}
	mkwServer.conn.Write(newPlayer.toBytes())
	logging.Info(moduleName, "Informing MKW-Server of new player", player.PlayerId)

	return nil
}

/*
MKW Sever expects this packet structure when a player leaves (or dcs) a froom

	type LeaveFroomMessage struct {
	    Id			LeaveFroom (0x02)
		addr		uint32
		port 		uint16
	}
*/
func (mkwServer *MKWServer) sendLeaveRoom(player *Player) {
	if mkwServer.conn == nil {
		logging.Error(moduleName, "MkwServerInfo.WfcMkwServerConn is nil. Cannot send remove client message")
		return
	}

	if player == nil {
		logging.Error(moduleName, "player is nil. Cannot send remove client message")
		return
	}

	ip, port := common.IPFormatToInt(player.Addr.String())

	pb := &common.PacketBuilder{Buf: make([]byte, 0, 7)}

	pb.WriteUint8(uint8(LeaveFroom))
	pb.WriteInt32(ip)
	pb.WriteUint16(port)

	logging.Info(moduleName, "sending leave room", pb.Buf)

	mkwServer.conn.Write(pb.Buf)
}
