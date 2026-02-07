package qr2

import (
	"fmt"

	"os/exec"
	"wwfc/common"
	"wwfc/logging"
)

const moduleName = "QR2 (MKW-Server Client Handler)"

func newMKWServerProxy(g *Group) *MKWServerProxy {
	// localhost if mkw-server is to spawn on the same machine, more logic would need to be added for remote mkw-server servers

	// for now just use the gamespy address
	roomAddress := *common.GetConfig().GameSpyAddress

	// use a random port to be less predictable and also avoid conflicts
	port, err := findOpenUDPPort(26000, 26999)
	if err != nil {
		logging.Error(moduleName, "Failed to find open UDP port:", err)
		return nil
	}

	roomAddress += ":" + fmt.Sprint(port)

	mkwServerPath := common.GetConfig().MkwServerPath

	logging.Info(moduleName, "Using mkw-server executable at", mkwServerPath)
	cmd := exec.Command(
		mkwServerPath,
		"--room-addr", roomAddress,
		"--wfc-addr", mkwServerListener.Addr().String(),
	)

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

	mkwServer := &MKWServerProxy{
		Cmd:             cmd,
		isRemote:        false,
		roomAddr:        nil, // set later by ROOM_OPEN message
		connToMKWServer: nil, // set later by ROOM_OPEN message
		GroupPointer:    g,
	}

	mkwServerProxies[roomAddress] = mkwServer

	logging.Info(moduleName, "Created new mkwServerProxy for room address", roomAddress)

	return mkwServer
}

// this will tell mkw-server to update its state since a player joined. it will send back the address players can connect to.
func (mkwServerProxy *MKWServerProxy) handlePlayerJoinFroomRequest(session *Session, buffer []byte) {
	logging.Info(moduleName, "Client (", session.Addr.String(), ") ", "requested to join a room")

	// client sends over {0xc, 0x2}, anything else is invalid
	if len(buffer) < 2 || buffer[0] != 0xc || buffer[1] != 0x2 {
		logging.Error(moduleName, "Invalid JOIN_FROOM request length")
		return
	}

	if mkwServerProxy.connToMKWServer == nil {
		logging.Error(moduleName, "MkwServerInfo.WfcMkwServerConn is nil. This shouldn't happen at this point")
		return
	}

	// message mkw-server expects is {0x2, 0x0, 0x0, 0x0} followed by the player's address
	message := make([]byte, 0)
	message = append(message, ServerJoinFroomRequest)
	message = append(message, 0x0)
	message = append(message, 0x0)
	message = append(message, 0x0)
	message = append(message, session.Addr.String()...)

	mkwServerProxy.connToMKWServer.Write(message)
}

func (mkwServerProxy *MKWServerProxy) sendMkwServerRemoveClient(session *Session) {
	if mkwServerProxy.connToMKWServer == nil {
		logging.Error(moduleName, "MkwServerInfo.WfcMkwServerConn is nil. Cannot send remove client message")
		return
	}

	if session == nil {
		logging.Error(moduleName, "Session is nil. Cannot send remove client message")
		return
	}

	// send over {0x3, 0x0, 0x0, 0x0} followed by the addr string
	message := make([]byte, 0)
	message = append(message, ServerLeaveFroomRequest)
	message = append(message, 0x0)
	message = append(message, 0x0)
	message = append(message, 0x0)
	message = append(message, session.Addr.String()...)

	mkwServerProxy.connToMKWServer.Write(message)
}
