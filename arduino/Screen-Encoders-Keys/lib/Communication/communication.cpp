#include "Communication.h"
#include <CRC8.h>

#define PACKET_HEADER 0xAA    // Example header byte
#define PACKET_FOOTER 0x55    // Example footer byte

Communication::Communication() {
    Serial.begin(9600);
}

void Communication::sendPacket(uint8_t command, uint8_t* payload, uint8_t length) {
    uint8_t packetLength = length + 2; // Length for payload + command + CRC
    uint8_t packet[packetLength + 2];  // +2 for header and footer

    packet[0] = PACKET_HEADER;
    packet[1] = packetLength;
    packet[2] = command;

    // Copy payload
    for (uint8_t i = 0; i < length; i++) {
        packet[3 + i] = payload[i];
    }

    // Calculate CRC
    crc8.reset();
    crc8.add(packet + 2, packetLength - 1); // CRC over command and payload
    uint8_t crc = crc8.getCRC();
    packet[3 + length] = crc;

    // Footer
    packet[4 + length] = PACKET_FOOTER;  // Footer should be here
    
    // Print packet contents for debugging
    for (uint8_t i = 0; i <= packetLength + 2; i++) {
        Serial.print("0x");
        if (packet[i] < 0x10) Serial.print("0");
        Serial.print(packet[i], HEX);
        Serial.print(" ");
    }
    Serial.println();
}

void Communication::receivePackage() {
    if (Serial.available() > 0) {
        uint8_t receivedByte = Serial.read();

        switch (state) {
            case WAIT_HEADER:
                if (receivedByte == PACKET_HEADER) {
                    state = GET_LENGTH;
                }
                break;

            case GET_LENGTH:
                packetLength = receivedByte;
                state = GET_COMMAND;
                break;

            case GET_COMMAND:
                commandByte = receivedByte;
                payloadIndex = 0;
                state = (packetLength > 3) ? GET_PAYLOAD : GET_CRC;
                break;

            case GET_PAYLOAD:
                payload[payloadIndex++] = receivedByte;
                if (payloadIndex == packetLength - 3) {
                    state = GET_CRC;
                }
                break;

            case GET_CRC:
                receivedCRC = receivedByte;
                state = GET_FOOTER;
                break;

            case GET_FOOTER:
                if (receivedByte == PACKET_FOOTER) {
                    crc8.reset();
                    crc8.add(&commandByte, 1); // Add the command byte to CRC
                    crc8.add(payload, packetLength - 3); // Add the payload to CRC
                    uint8_t calculatedCRC = crc8.getCRC();
                    if (receivedCRC == calculatedCRC) {
                        processCommand(commandByte, payload, packetLength - 3);
                        sendAcknowledge(true); // Should be called here
                    } else {
                        sendAcknowledge(false); // Should be called here
                    }
                }
                state = WAIT_HEADER;
                break;
        }
    }
}

void Communication::processCommand(uint8_t command, uint8_t* payload, uint8_t length) {
    Serial.print("Processing Command: ");
    Serial.println(command, HEX);

    switch (command) {
        case RECEIVED_CONFIG:
            configRecieved = true;
            Serial.println("Received Config Command");
            // Process RECEIVED_CONFIG
            break;
        case UPDATE_VOLUME:
            Serial.println("Received Update Volume Command");
            // Process UPDATE_VOLUME
            break;
        case CMD_ANOTHER_COMMAND:
            Serial.println("Received Another Command");
            // Process CMD_ANOTHER_COMMAND
            break;
        // Add more cases as needed
    }
}

void Communication::sendAcknowledge(bool success) {
    uint8_t ackPayload[1];
    ackPayload[0] = success ? 0x01 : 0x00; 
    if (success){
        // uint8_t ackPayload[0] = {0x01};
        sendPacket( ACKNOWLEDGE , ackPayload, 1);
    } else {
        // uint8_t ackPayload[0] = {0x00};
        sendPacket( ACKNOWLEDGE , ackPayload, 1);
    }
    
}
