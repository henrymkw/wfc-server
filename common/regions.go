package common

type Region int

// Room Regions clients can search for. Custom regions can be added here.
const (
	None = 0x00 // WorldWide or
	NA   = 0x01
	EU   = 0x02
	JP   = 0x03
	KOR  = 0x04
)
