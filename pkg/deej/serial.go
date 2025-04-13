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

	if err := sio.initializeConnection(); err != nil {
		return err
	}
	// Start reading from the connection
	//go sio.resumeReading()

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

func (sio *SerialIO) resumeReading() {
	namedLogger := sio.logger.Named(strings.ToLower(sio.connOptions.PortName))
	connReader := bufio.NewReader(sio.conn)
	lineChannel := sio.readBytes(namedLogger, connReader)

	for {
		select {
		case <-sio.stopChannel:
			sio.close(namedLogger)
			return
		case line := <-lineChannel:
			sio.handleBytes(line)
		}
	}
}

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
	sio.logger.Info("Sending packet", "command:", command, " payload:", payload, " crc:", crc)
	sio.logger.Info("Full packet: ", packet)
	// Send the packet over the serial connection

	if _, err := sio.conn.Write(packet); err != nil {
		sio.logger.Warnw("Failed to send packet to serial", "error", err)
		return err
	} else {
	}

	sio.logger.Debug("Packet sent successfully")
	return nil
}

func (sio *SerialIO) initializeConnection() error {
	payloadChunk := []byte{0x99}

	// Log the chunk being sent
	sio.logger.Info("Preparing to send configuration packet chunk: ", payloadChunk)

	// Send the payload chunk to Arduino
	sio.logger.Info("Sending configuration packet to Arduino (chunked)")
	err := sio.sendPacket(CMD_ANOTHER_COMMAND, payloadChunk)
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

	sio.logger.Info("Connection initialized successfully with all configuration data")
	return nil
}

func (sio *SerialIO) listenForAck(ackChannel chan bool) {
	namedLogger := sio.logger.Named(strings.ToLower(sio.connOptions.PortName))
	connReader := bufio.NewReader(sio.conn)
	lineChannel := sio.readBytes(namedLogger, connReader)

	for {
		select {
		case <-sio.stopChannel:
			sio.close(namedLogger)
			return
		case line := <-lineChannel:
			// Handle the command
			command, payload, MatchCRC := sio.ParsePacket(line)
			if CommandType(command) == ACKNOWLEDGE && MatchCRC {
				sio.logger.Info("Received ACKNOWLEDGE command", "payload", payload)
				ackChannel <- true
			} else {
				sio.logger.Info("Received non-ACKNOWLEDGE command")
				ackChannel <- false
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

func (sio *SerialIO) handleBytes(data []byte) {
	// Log de ontvangen bytes
	sio.logger.Info("Handling received bytes:", data)

	// Controleer of het pakket geldig is
	if len(data) < 6 { // Minimale pakketgrootte: header + lengte + commando + CRC + footer
		sio.logger.Info("Received packet too short to be valid")
		return
	}

	if data[0] != PACKET_HEADER || data[len(data)-1] != PACKET_FOOTER {
		sio.logger.Info("Invalid packet structure")
		return
	}

	// Parse het pakket
	command, payload, MatchCRC := sio.ParsePacket(data)
	sio.logger.Info("Parsed packet details", "command", command, "payload", payload, "MatchCRC", MatchCRC)

	if MatchCRC {
		// sio.sendPacket(ACKNOWLEDGE, []byte{1})
		sio.logger.Info("CRC correct")
		sio.handlePayload(command, payload)
	} else {
		sio.logger.Info("CRC mismatch")
		// sio.sendPacket(ACKNOWLEDGE, []byte{0})
	}
}

func (sio *SerialIO) readBytes(logger *zap.SugaredLogger, reader *bufio.Reader) chan []byte {
	ch := make(chan []byte)

	go func() {
		buffer := make([]byte, 6) // Pas de bufferlengte aan op basis van je pakketgrootte
		for {
			n, err := reader.Read(buffer)
			if err != nil {
				if sio.deej.Verbose() {
					logger.Warnw("Failed to read bytes from serial", "error", err)
				}
				return
			}

			if sio.deej.Verbose() {
				logger.Debugw("Read new bytes", "bytes", buffer[:n])
			}

			// Stuur de gelezen bytes naar het kanaal
			ch <- buffer[:n]
		}
	}()

	return ch
}
