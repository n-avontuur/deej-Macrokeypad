package deej

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/jacobsa/go-serial/serial"
	"go.uber.org/zap"

	"github.com/omriharel/deej/pkg/deej/util"
	"github.com/sigurn/crc8"
)

// SerialIO provides a deej-aware abstraction layer to managing serial I/O
type SerialIO struct {
	comPort  string
	baudRate uint

	deej   *Deej
	logger *zap.SugaredLogger

	stopChannel chan bool
	connected   bool
	connOptions serial.OpenOptions
	conn        io.ReadWriteCloser

	lastKnownNumSliders        int
	currentSliderPercentValues []float32

	sliderMoveConsumers []chan SliderMoveEvent
}

// SliderMoveEvent represents a single slider move captured by deej
type SliderMoveEvent struct {
	SliderID     int
	PercentValue float32
}

var expectedLinePattern = regexp.MustCompile(`^\d{1,4}(\|\d{1,4})*\r\n$`)

const (
	PACKET_HEADER = 0xAA
	PACKET_FOOTER = 0x55
	CRC8_POLY     = 0x07
)

// NewSerialIO creates a SerialIO instance that uses the provided deej
// instance's connection info to establish communications with the arduino chip
func NewSerialIO(deej *Deej, logger *zap.SugaredLogger) (*SerialIO, error) {
	logger = logger.Named("serial")

	sio := &SerialIO{
		deej:                deej,
		logger:              logger,
		stopChannel:         make(chan bool),
		connected:           false,
		conn:                nil,
		sliderMoveConsumers: []chan SliderMoveEvent{},
	}

	logger.Debug("Created serial i/o instance")

	// respond to config changes
	sio.setupOnConfigReload()

	return sio, nil
}

// Start attempts to connect to our arduino chip
func (sio *SerialIO) Start() error {

	// don't allow multiple concurrent connections
	if sio.connected {
		sio.logger.Warn("Already connected, can't start another without closing first")
		return errors.New("serial: connection already active")
	}

	// set minimum read size according to platform (0 for windows, 1 for linux)
	// this prevents a rare bug on windows where serial reads get congested,
	// resulting in significant lag
	minimumReadSize := 0
	if util.Linux() {
		minimumReadSize = 1
	}

	sio.connOptions = serial.OpenOptions{
		PortName:        sio.deej.config.ConnectionInfo.COMPort,
		BaudRate:        uint(sio.deej.config.ConnectionInfo.BaudRate),
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: uint(minimumReadSize),
	}

	sio.logger.Debugw("Attempting serial connection",
		"comPort", sio.connOptions.PortName,
		"baudRate", sio.connOptions.BaudRate,
		"minReadSize", minimumReadSize)

	var err error
	sio.conn, err = serial.Open(sio.connOptions)
	if err != nil {

		// might need a user notification here, TBD
		sio.logger.Warnw("Failed to open serial connection", "error", err)
		return fmt.Errorf("open serial connection: %w", err)
	}

	namedLogger := sio.logger.Named(strings.ToLower(sio.connOptions.PortName))

	namedLogger.Infow("Connected", "conn", sio.conn)
	sio.connected = true

	sio.initializeConnection()

	// Start reading from the connection
	go sio.resumeReading()

	return nil
}

// Stop signals us to shut down our serial connection, if one is active
func (sio *SerialIO) Stop() {
	if sio.connected {
		sio.logger.Debug("Shutting down serial connection")
		sio.stopChannel <- true
	} else {
		sio.logger.Debug("Not currently connected, nothing to stop")
	}
}

// SubscribeToSliderMoveEvents returns an unbuffered channel that receives
// a sliderMoveEvent struct every time a slider moves
func (sio *SerialIO) SubscribeToSliderMoveEvents() chan SliderMoveEvent {
	ch := make(chan SliderMoveEvent)
	sio.sliderMoveConsumers = append(sio.sliderMoveConsumers, ch)

	return ch
}

func (sio *SerialIO) setupOnConfigReload() {
	configReloadedChannel := sio.deej.config.SubscribeToChanges()

	const stopDelay = 50 * time.Millisecond

	go func() {
		for range configReloadedChannel {

			// make any config reload unset our slider number to ensure process volumes are being re-set
			// (the next read line will emit SliderMoveEvent instances for all sliders)\
			// this needs to happen after a small delay, because the session map will also re-acquire sessions
			// whenever the config file is reloaded, and we don't want it to receive these move events while the map
			// is still cleared. this is kind of ugly, but shouldn't cause any issues
			go func() {
				<-time.After(stopDelay)
				sio.lastKnownNumSliders = 0
			}()

			// if connection params have changed, attempt to stop and start the connection
			if sio.deej.config.ConnectionInfo.COMPort != sio.connOptions.PortName ||
				uint(sio.deej.config.ConnectionInfo.BaudRate) != sio.connOptions.BaudRate {

				sio.logger.Info("Detected change in connection parameters, attempting to renew connection")
				sio.Stop()

				// let the connection close
				<-time.After(stopDelay)

				if err := sio.Start(); err != nil {
					sio.logger.Warnw("Failed to renew connection after parameter change", "error", err)
				} else {
					sio.logger.Debug("Renewed connection successfully")
				}
			}
		}
	}()
}

func (sio *SerialIO) close(logger *zap.SugaredLogger) {
	if err := sio.conn.Close(); err != nil {
		logger.Warnw("Failed to close serial connection", "error", err)
	} else {
		logger.Debug("Serial connection closed")
	}

	sio.conn = nil
	sio.connected = false
}

func (sio *SerialIO) readLine(logger *zap.SugaredLogger, reader *bufio.Reader) chan string {
	ch := make(chan string)

	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {

				if sio.deej.Verbose() {
					logger.Warnw("Failed to read line from serial", "error", err, "line", line)
				}

				// just ignore the line, the read loop will stop after this
				return
			}

			if sio.deej.Verbose() {
				logger.Debugw("Read new line", "line", line)
			}

			// deliver the line to the channel
			ch <- line
		}
	}()

	return ch
}

func (sio *SerialIO) sendLine(line string) error {
	if !sio.connected {
		return errors.New("serial: not connected")
	}

	// Stop reading temporarily by closing the current reader.
	sio.stopChannel <- true

	// Send the line over the serial connection.
	_, err := sio.conn.Write([]byte(line + "\r\n"))
	if err != nil {
		sio.logger.Warnw("Failed to send line to serial", "error", err, "line", line)
		return err
	}

	// Resume reading after sending the data.
	go sio.resumeReading()

	return nil
}

func (sio *SerialIO) resumeReading() {
	namedLogger := sio.logger.Named(strings.ToLower(sio.connOptions.PortName))
	connReader := bufio.NewReader(sio.conn)
	lineChannel := sio.readLine(namedLogger, connReader)

	for {
		select {
		case <-sio.stopChannel:
			sio.close(namedLogger)
			return
		case line := <-lineChannel:
			sio.handleLine(namedLogger, line)
		}
	}
}

func (sio *SerialIO) handleLine(logger *zap.SugaredLogger, line string) {

	fmt.Printf("%s", line)

	bytes, err := ConvertHexStringToBytes(line)
	// for i := 0; i < len(bytes); i++ {
	// 	fmt.Printf("data[%d]: [0x%02X] || ", i, bytes[i])
	// }
	// fmt.Println("!! END !!")

	// Check for conversion errors
	if err != nil {
		logger.Warn("Error during conversion: %v", err)
		return
	}

	// Check if the received line is too short
	if len(bytes) < 5 { // Minimum packet size: header + length + command + CRC + footer
		logger.Warn("Received line too short to be a valid packet")
		return
	}

	// Check for valid header and footer
	if bytes[0] != PACKET_HEADER {
		logger.Warn("Invalid PACKET_HEADER structure")
		return
	}

	if bytes[len(bytes)-1] != PACKET_FOOTER {
		logger.Warn("Invalid PACKET_FOOTER structure")
		return
	}

	command, payload, MatchCRC := ParsePacket(bytes)
	//fmt.Printf("command, payload, MatchCRC : %x, %x, %s", command, payload, MatchCRC)
	if MatchCRC {
		CorrectMatch := []byte{1}
		sio.sendPacket(ACKNOWLEDGE, CorrectMatch)
		handlePayload(command, payload)
		return
	} else {
		CorrectMatch := []byte{0}
		logger.Warn("CRC mismatch")
		sio.sendPacket(ACKNOWLEDGE, CorrectMatch)
	}

	//// Temperatie removed, should be added back in to
	// var encoderLines []string

	// // for each slider:
	// for sliderIdx, stringValue := range encoderLines {

	// 	// convert string values to integers ("1023" -> 1023)
	// 	number, error := strconv.Atoi(stringValue)
	// 	if error != nil {
	// 		return
	// 	}

	// 	// turns out the first line could come out dirty sometimes (i.e. "4558|925|41|643|220")
	// 	// so let's check the first number for correctness just in case
	// 	if sliderIdx == 0 && number > 1023 {
	// 		sio.logger.Debugw("Got malformed line from serial, ignoring", "line", line)
	// 		return
	// 	}

	// 	// map the value from raw to a "dirty" float between 0 and 1 (e.g. 0.15451...)
	// 	dirtyFloat := float32(number) / 1023.0

	// 	// normalize it to an actual volume scalar between 0.0 and 1.0 with 2 points of precision
	// 	normalizedScalar := util.NormalizeScalar(dirtyFloat)

	// 	// if sliders are inverted, take the complement of 1.0
	// 	if sio.deej.config.InvertSliders {
	// 		normalizedScalar = normalizedScalar - 1
	// 	}

	// 	if sio.currentSliderPercentValues[sliderIdx] == normalizedScalar {
	// 		return
	// 	}

	// 	if Encoders[sliderIdx].functionName == "controlVolume" {
	// 		volumeDifference := sio.currentSliderPercentValues[sliderIdx] - normalizedScalar
	// 		//fmt.Printf("Set new volume %f for slider[%d] \n ", sio.currentSliderPercentValues[sliderIdx], sliderIdx)
	// 		if volumeDifference <= 5 || volumeDifference >= -5 {
	// 			Encoders[sliderIdx].function(sio.deej, sliderIdx, normalizedScalar)
	// 		} else {
	// 			sio.sendLine("625|625") // Should be extended is not correct now.
	// 		}
	// 		return
	// 	} else {
	// 		fmt.Printf(" function name is : %s \n", Encoders[sliderIdx].functionName)
	// 	}
	// }
}

// ConvertHexStringToBytes converts a space-separated hex string to a byte slice.
func ConvertHexStringToBytes(hexStr string) ([]byte, error) {
	// Split the string into individual hex values
	hexStrings := strings.Fields(hexStr) // Split by whitespace

	// Create a byte slice to store the result
	bytes := make([]byte, len(hexStrings))

	// Convert each hex value to a byte
	for i, hex := range hexStrings {
		// Remove the '0x' prefix if present
		hex = strings.TrimPrefix(hex, "0x")

		// Check if the hex string is valid
		if len(hex) != 2 {
			return nil, fmt.Errorf("invalid hex value: %s", hex)
		}

		// Parse the hex string as a byte
		var b byte
		_, err := fmt.Sscanf(hex, "%2X", &b)
		if err != nil {
			return nil, fmt.Errorf("error parsing hex value %s: %v", hex, err)
		}

		// Store the byte in the slice
		bytes[i] = b
	}

	return bytes, nil
}

// added function for two way communication

func (sio *SerialIO) sendPacket(command CommandType, payload []byte) error {
	if !sio.connected {
		return errors.New("serial: not connected")
	}

	packetLength := uint8(len(payload) + 2) // Command + CRC
	packet := make([]byte, packetLength+3)  // Header + Length + Command + Payload + CRC + Footer
	packet[0] = PACKET_HEADER
	packet[1] = packetLength
	packet[2] = byte(command)
	copy(packet[3:], payload)

	// Calculate CRC for the packet
	crc := crc8.Checksum(packet[2:packetLength+1], crc8.MakeTable(crc8.CRC8_MAXIM))
	packet[3+len(payload)] = crc
	packet[4+len(payload)] = PACKET_FOOTER

	// Log the packet details
	sio.logger.Debugw("Sending packet", "command", command, "payload", payload, "crc", crc)

	// Send the packet over the serial connection
	if _, err := sio.conn.Write(packet); err != nil {
		sio.logger.Warnw("Failed to send packet to serial", "error", err)
		return err
	}

	sio.logger.Debug("Packet sent successfully")
	return nil
}

func (sio *SerialIO) initializeConnection() error {
	const maxPacketSize = 64 // Set this according to the maximum payload size Arduino can handle
	serializedPages, err := json.Marshal(sio.deej.config.Pages)
	if err != nil {
		sio.logger.Warn("Failed to serialize configuration data", "error", err)
		return err
	}

	// Divide serializedPages into chunks if necessary
	for i := 0; i < len(serializedPages); i += maxPacketSize {
		end := i + maxPacketSize
		if end > len(serializedPages) {
			end = len(serializedPages)
		}

		// Extract chunk of payload
		payloadChunk := serializedPages[i:end]

		// Log the chunk being sent
		sio.logger.Info("Preparing to send configuration packet chunk", "start", i, "end", end, "chunk", string(payloadChunk))

		// Send the payload chunk to Arduino
		sio.logger.Info("Sending configuration packet to Arduino (chunked)")
		err := sio.sendPacket(CONFIG_NEEDED, payloadChunk)
		if err != nil {
			sio.logger.Warn("Failed to send configuration packet chunk", "error", err)
			return err
		}

		const maxRetries = 3

	ackLoop:
		for attempt := 1; attempt <= maxRetries; attempt++ {
			sio.logger.Infow("Waiting for acknowledgment", "attempt", attempt, "maxRetries", maxRetries)

			ackChannel := make(chan bool)
			go sio.listenForAck(ackChannel)

			// Wait for acknowledgment with a timeout
			select {
			case ack := <-ackChannel:
				if ack {
					sio.logger.Info("Received acknowledgment for this chunk")
					break ackLoop // Acknowledgment ontvangen, stop met proberen
				} else {
					sio.logger.Warnw("Failed to receive acknowledgment for this chunk", "attempt", attempt)
				}
			case <-time.After(10 * time.Second): // Verhoog timeout naar 10 seconden
				sio.logger.Warnw("Timeout waiting for acknowledgment from Arduino", "attempt", attempt)
			}
			// Als dit de laatste poging is, escaleer naar een fout
			if attempt == maxRetries {
				sio.logger.Error("Exceeded maximum retries for acknowledgment")
				return errors.New("failed to receive acknowledgment after maximum retries")
			}

			// Optioneel: Wacht een korte tijd voordat je opnieuw probeert
			time.Sleep(1 * time.Second)
		}
	}

	sio.logger.Info("Connection initialized successfully with all configuration data")
	return nil
}

func (sio *SerialIO) listenForAck(ackChannel chan bool) {
	namedLogger := sio.logger.Named(strings.ToLower(sio.connOptions.PortName))
	connReader := bufio.NewReader(sio.conn)
	lineChannel := sio.readLine(namedLogger, connReader)

	for {
		select {
		case <-sio.stopChannel:
			sio.close(namedLogger)
			return
		case line := <-lineChannel:
			// Parse incoming packet/line
			sio.logger.Info("Received line from Arduino ", "line: ", line)
			command, payload, MatchCRC := ParsePacket([]byte(line))
			sio.logger.Debugw("Parsed packet details",
				"header", line[0],
				"length", line[1],
				"command", line[2],
				"payload", line[3:len(line)-2],
				"crc", line[len(line)-2],
				"footer", line[len(line)-1],
			)
			sio.logger.Info("Length of payload: ", len(line), " | ", len(payload))
			sio.logger.Info("Parsed packet", " | command: ", command, " | payload: ", payload, " | MatchCRC: ", MatchCRC)
			if CommandType(command) == ACKNOWLEDGE && len(payload) > 0 && payload[0] == 1 && MatchCRC {
				sio.logger.Info("Received acknowledgment from Arduino")
				ackChannel <- true
				return
			} else if CommandType(command) == ACKNOWLEDGE && len(payload) > 0 && payload[0] == 0 && !MatchCRC {
				sio.logger.Warn("Arduino reported an error in acknowledgment")
				ackChannel <- false
				return
			} else {
				sio.logger.Warnw("Unexpected response from Arduino")
			}
		}
	}
}

func (sio *SerialIO) sendPagesToArduino() error {
	pages := sio.deej.config.Pages
	jsonData, err := json.Marshal(pages)

	sio.stopChannel <- true
	// Send the line over the serial connection.
	_, err = sio.conn.Write(jsonData)
	if err != nil {
		sio.logger.Warnw("Failed to send line to serial", "error", err, "jsonData", jsonData)
		return err
	}
	return nil
}

func (sio *SerialIO) HandleEncoderValues(line string) {
	//var encoderLines []string
	if !expectedLinePattern.MatchString(line) {
		fmt.Printf("Line not matching pattern")
		return
	}

	// trim the suffix
	line = strings.TrimSuffix(line, "\r\n")

	// split on pipe (|), this gives a slice of numerical strings between "0" and "1023"
	splitLine := strings.Split(line, "|")

	//Added code so that lasted element gets split of. This because the last element is the key for sending commands.
	if len(splitLine) > 0 {
		//This was to for splitting the command from the slider values
		lastIdx := len(splitLine) - 1
		lastElement := splitLine[lastIdx]
		sio.deej.receiveKey(lastElement)
		//remove the last element because this it the key-command

		if len(splitLine) > 0 {
			encoderLines := splitLine[:len(splitLine)-1]
			fmt.Printf("encoderLine: %s", encoderLines)
		}
	}
	numberOfMappedSliders := 0
	sio.deej.config.SliderMapping.iterate(func(sliderIdx int, slider []string) {
		numberOfMappedSliders += 1
	})

	// update our slider count, if needed - this will send slider move events for all
	if numberOfMappedSliders != sio.lastKnownNumSliders {
		setupEncoderAmount(numberOfMappedSliders)
		fmt.Printf("Detected sliders", "amount", numberOfMappedSliders)
		sio.lastKnownNumSliders = numberOfMappedSliders
		sio.currentSliderPercentValues = make([]float32, numberOfMappedSliders)
		// reset everything to be an impossible value to force the slider move event later
		for idx := range sio.currentSliderPercentValues {
			sio.currentSliderPercentValues[idx] = -1.0
		}
		fmt.Printf("last know number of sliders : %f", sio.currentSliderPercentValues)
	}
}
