package deej

import "fmt"

type CommandType byte

const (
	ACKNOWLEDGE         CommandType = 0x00
	CONFIG_NEEDED       CommandType = 0x01
	UPDATE_VOLUME       CommandType = 0x02
	CMD_ANOTHER_COMMAND CommandType = 0x03
)

func handlePayload(command byte, payload []byte) {
	// Handle the command
	switch CommandType(command) {
	case ACKNOWLEDGE:
		fmt.Println("Message ACKNOWLEDGE")
		// Process RECEIVED_CONFIG command
	case CONFIG_NEEDED:
		fmt.Println("Config needed")
		// Process RECEIVED_CONFIG command
	case UPDATE_VOLUME:
		fmt.Println("Update volume command")
		// Process UPDATE_VOLUME command
	case CMD_ANOTHER_COMMAND:
		fmt.Println("Another command received")
		// Process CMD_ANOTHER_COMMAND command
	default:
		fmt.Println("Unknown command received")
	}
}
