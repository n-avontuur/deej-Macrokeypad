package deej

import (
	"github.com/sigurn/crc8"
)

// CRC8 function calculates CRC-8 using polynomial 0x07
func calculateCRC8(data []byte) byte {
	var crc byte = 0xFF
	for _, b := range data {
		crc ^= b
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ CRC8_POLY
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func (sio *SerialIO) ParsePacket(packet []byte) (byte, []byte, bool) {
	length := len(packet) - 2
	// Controleer minimale lengte (header + lengte + commando + CRC + footer)
	if len(packet) < 5 {
		return 99, nil, false
	}
	// Controleer header en footer
	if packet[0] != 170 || packet[length] != 85 {
		sio.logger.Info("Invalid packet: %v | %v\n", packet[0], packet[length])

		return 88, nil, false
	}

	if len(packet) != length+2 {
		return 77, nil, false
	}

	// Lees commando en payload
	command := packet[2]
	payload := packet[3 : len(packet)-2]

	// Controleer CRC
	crc := packet[len(packet)-2]
	calculatedCRC := crc8.Checksum(packet[2:length-2], crc8.MakeTable(crc8.CRC8_MAXIM))
	if crc != calculatedCRC {
		return command, payload, false
	}

	return command, payload, true
}
