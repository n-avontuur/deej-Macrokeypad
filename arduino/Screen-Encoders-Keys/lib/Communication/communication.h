#ifndef COMMUNICATION_H
#define COMMUNICATION_H

#include <CRC8.h>



enum CommandType {
    ACKNOWLEDGE         = 0x00,
	RECEIVED_CONFIG     = 0x01,
	UPDATE_VOLUME       = 0x02,
	CMD_ANOTHER_COMMAND = 0x03
};

enum ReceiverState {
    WAIT_HEADER,
    GET_LENGTH,
    GET_COMMAND,
    GET_PAYLOAD,
    GET_CRC,
    GET_FOOTER
};

class Communication {
public:
    Communication();  // Constructor declaration
    void sendPacket(uint8_t command, uint8_t* payload, uint8_t length);
    void receivePackage();
    bool configRecieved = false;

private:
    void processCommand(uint8_t command, uint8_t* payload, uint8_t length);
    void sendAcknowledge(bool success);

    ReceiverState state = WAIT_HEADER;
    uint8_t packetLength = 0;
    uint8_t commandByte = 0;
    uint8_t payload[255];
    uint8_t payloadIndex = 0;
    uint8_t receivedCRC = 0;
    CRC8 crc8;  // Moved to private member
};

#endif // COMMUNICATION_H
