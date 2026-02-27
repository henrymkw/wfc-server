package qr2

import (
	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
	"github.com/sasha-s/go-deadlock"
)

var ServerName = "roommanager"
var rooms = map[string]*Room{}

const (
	// Requests sent from the client
	PlaceHolderRequest = 0x00
)

var (
	connBuffers = map[uint64]*[]byte{}
	mutexRM       = deadlock.RWMutex{}

)

func NewConnection(index uint64, address string) {}

func CloseConnection(index uint64) {
	mutexRM.Lock()
	delete(connBuffers, index)
	mutexRM.Unlock()
}

func HandlePacket(index uint64, data []byte, address string) {
	moduleName := "RM:" + address

	logging.Info(moduleName, "Received packet with length", aurora.Cyan(len(data)), "from connection index", aurora.Cyan(index), "with data:", aurora.Cyan(printHex(data)))

	mutexRM.RLock()
	buffer := connBuffers[index]
	mutexRM.RUnlock()

	if buffer == nil {
		buffer = &[]byte{}
		defer func() {
			if buffer == nil {
				return
			}

			mutexRM.Lock()
			connBuffers[index] = buffer
			mutexRM.Unlock()
		}()
	}

	if len(*buffer)+len(data) > 0x1000 {
		logging.Error(moduleName, "Buffer overflow")
		common.CloseConnection(ServerName, index)
		buffer = nil
		return
	}
}
