#include "Communication.h"

Communication comm;

void setup() {
    // Initialize Serial communication
    Serial.begin(9600);

    // Wait for the Serial Monitor to open
    while (!Serial) {
        ; // Wait for Serial to be ready
    }

    // Print a message to the Serial Monitor
    Serial.println("Communication Test Starting...");

    while (!comm.configRecieved) {
        comm.receivePackage();
        delay(1000); // Wait for a second before checking again
    }


}


void loop() {
    // Example payload: the string "Spotify"
    // uint8_t simplePayload[] = {"Spotify"};
    // comm.sendPacket(0x01, simplePayload, 1);

    // delay(5000);
    
    
}