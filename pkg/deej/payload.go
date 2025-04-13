package deej

type CommandType byte

const (
	ACKNOWLEDGE         CommandType = 0x00
	CONFIG_NEEDED       CommandType = 0x01
	UPDATE_VOLUME       CommandType = 0x02
	CMD_ANOTHER_COMMAND CommandType = 0x03
)

func (sio SerialIO) handlePayload(command byte, payload []byte) {
	// Handle the command
	switch CommandType(command) {
	case ACKNOWLEDGE:
		sio.logger.Info("Message ACKNOWLEDGE")
		// Process RECEIVED_CONFIG command
	case CONFIG_NEEDED:
		sio.logger.Info("Config needed")
		// Process RECEIVED_CONFIG command
	case UPDATE_VOLUME:
		sio.logger.Info("Update volume command")
		// Process UPDATE_VOLUME command
	case CMD_ANOTHER_COMMAND:
		sio.logger.Info("Another command received")
		// Process CMD_ANOTHER_COMMAND command
	default:
		sio.logger.Info("Unknown command received")
	}
}
