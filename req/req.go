package req

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/kristhecanadian/kurl/cli"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	ACK    uint8 = 0
	SYN    uint8 = 1
	FIN    uint8 = 2
	NACK   uint8 = 3
	SYNACK uint8 = 4
	DATA   uint8 = 5
)

type Response struct {
	Proto      string
	StatusCode int
	Headers    map[string]string
	Body       string
}

type encodedMessage struct {
	packetType     [1]byte
	sequenceNumber [4]byte
	peerAddress    [4]byte
	peerPort       [2]byte
	// Size 1013
	payload []byte
}

type message struct {
	packetType     uint8
	sequenceNumber uint32
	peerAddress    string
	peerPort       uint16
	payload        string
}

func Request(opts *cli.Options) (res Response, resString string) {
	u, host, port, req := creatingHttpRequest(opts)
	port = parseProtocol(u, port)

	routerPort := opts.Port

	split := strings.Split(host, ":")
	host = split[0]

	address := "" + host + ":" + strconv.Itoa(routerPort)

	udpAddr, err := net.ResolveUDPAddr("udp", address)
	checkError(&err)

	udpConn, bMessage := initializeHandshake(udpAddr, port, host)
	checkError(&err)

	udpConn, bMessage, port = handleHandshake(address, udpConn, udpAddr, port, host, bMessage)

	frames := make(map[int]encodedMessage)
	ackedFrames := make(map[int]bool)
	breq := []byte(req)

	chunkDataIntoFrames(breq, udpConn, frames, ackedFrames, port)

	bMessage = sendFrames(frames, bMessage, udpConn, ackedFrames)

	handleFrameAcks(err, udpConn, ackedFrames, frames, bMessage, port, host)

	resFrames := handleResFrames(udpConn, port)
	sbPayload := parseFrames(resFrames)

	res = Response{}

	ParseResponse(sbPayload.String(), &res, &resString)

	return res, resString

}

func handleFrameAcks(err error, udpConn *net.UDPConn, ackedFrames map[int]bool, frames map[int]encodedMessage, bMessage bytes.Buffer, port string, host string) {
	// START WAITING FOR THOSE ACK FRAMES
	err = udpConn.SetReadDeadline(time.Now().Add(8 * time.Second))
	checkError(&err)
	count := 10
	for {
		buf := make([]byte, 1024)
		n, addr, err := udpConn.ReadFromUDP(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			isAllAcked := true
			// CHECK ACKED Frames
			for frameNumber, isAcked := range ackedFrames {
				isAllAcked = isAllAcked && isAcked
				if !isAcked {
					frameMessage := frames[frameNumber]
					bMessage = convertMessageToBytes(frameMessage)
					_, err = udpConn.Write(bMessage.Bytes())
					checkError(&err)
				}
			}
			if isAllAcked {
				// ALL PACKETS HAVE BEEN ACKED.
				break
			}
			// RESET TIMER
			err = udpConn.SetReadDeadline(time.Now().Add(8 * time.Second))
			count--
			if count < 0 {
				break
			}
			continue
		}
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				continue
			}
		}

		// CHECK FOR ACK
		m := parseMessage(buf, n)

		log.Println("Received a " + strconv.Itoa(int(m.packetType)) + " packet as a response! from " + addr.String())

		if m.packetType == SYNACK {
			handleSYNACKAfterConnection(port, host, udpConn)
			continue
		}

		if m.packetType == DATA {
			break
		}

		if m.packetType != ACK {
			// ignore it and continue waiting
			continue
		}
		// CHECK IF SEQUENCE MATCHES A PACKET SENT
		frameNumber := int(m.sequenceNumber)
		if ackedFrames[frameNumber] {
			// We already received this
			continue
		}
		log.Println("Ack packet received for frame:" + strconv.Itoa(frameNumber))
		ackedFrames[frameNumber] = true
		err = udpConn.SetReadDeadline(time.Now().Add(8 * time.Second))
	}
}

func handleSYNACKAfterConnection(port string, host string, udpConn *net.UDPConn) {
	// ACK FOR STARTING CONNECTION MUST HAVE DROPPED
	// LET US ACK IT
	payloadBuffer := make([]byte, 50)
	payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
	payloadBufferWriter.WriteString("0")

	intPort, _ := strconv.Atoi(port)

	ackMessage := createMessage(ACK, 10, host, intPort, payloadBuffer)
	bAckMessage := convertMessageToBytes(ackMessage)

	_, err := udpConn.Write(bAckMessage.Bytes())
	checkError(&err)
	err = udpConn.SetReadDeadline(time.Now().Add(8 * time.Second))
}

func sendFrames(frames map[int]encodedMessage, bMessage bytes.Buffer, udpConn *net.UDPConn, ackedFrames map[int]bool) bytes.Buffer {
	// SEND THE MESSAGE FRAMES
	for frameNumber, frameMessage := range frames {
		// encode that frame
		bMessage = convertMessageToBytes(frameMessage)
		// send it to the client
		_, err := udpConn.Write(bMessage.Bytes())
		checkError(&err)
		ackedFrames[frameNumber] = false
	}
	log.Println("Sent all frames!")
	return bMessage
}

func chunkDataIntoFrames(bRes []byte, udpConn *net.UDPConn, resFrames map[int]encodedMessage, ackedFrames map[int]bool, port string) {
	chunks := split(bRes, 1013)
	frameNumber := 10

	// SPLIT PAYLOAD INTO FRAMES
	for _, chunk := range chunks {
		ipAddress, _ := connectionGetPortAndIP(udpConn)
		sPort, err := strconv.Atoi(port)
		checkError(&err)
		m := createMessage(DATA, frameNumber, ipAddress, sPort, chunk)
		resFrames[frameNumber] = m
		ackedFrames[frameNumber] = false
		frameNumber++
	}
}

func parseFrames(frames map[int]message) strings.Builder {
	// We now have all the frames
	// Let us get theses messages and reconstruct the payload
	frameNumbers := make([]int, len(frames))
	i := 0
	for number, _ := range frames {
		frameNumbers[i] = number
		i++
	}
	sort.Ints(frameNumbers)

	// Reconstruct the payload
	sbPayload := strings.Builder{}
	for _, Number := range frameNumbers {
		sbPayload.WriteString(frames[Number].payload)
	}
	return sbPayload
}

func handleResFrames(udpConn *net.UDPConn, port string) map[int]message {
	frames := make(map[int]message)
	err := udpConn.SetReadDeadline(time.Now().Add(6 * time.Second))
	checkError(&err)
	firstTimeReceived := false
	for {
		buf := make([]byte, 1024)
		n, err := udpConn.Read(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			err = udpConn.SetReadDeadline(time.Now().Add(6 * time.Second))
			// if you received packets then timed out then break
			if firstTimeReceived {
				// check if there's a sequence missing before breaking
				frameNumbers := make([]int, len(frames))
				i := 0
				for number, _ := range frames {
					frameNumbers[i] = number
					i++
				}
				sort.Ints(frameNumbers)
				length := len(frameNumbers)
				dif := frameNumbers[len(frameNumbers)-1] - frameNumbers[0]
				if length-1 == dif {
					break
				}

			}
			// just keep waiting for client to send stuff
			continue
		}
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				continue
			}
		}
		// CHECK FOR DATA
		m := parseMessage(buf, n)

		if m.packetType == SYNACK {
			handleSYNACKAfterConnection(port, udpConn.LocalAddr().String(), udpConn)
			continue
		}

		if m.packetType != DATA {
			// ignore it and continue waiting
			continue
		}
		// set first time received to true
		firstTimeReceived = true

		// check to see if the frame is already there first
		frames[int(m.sequenceNumber)] = m
		ipAddress, _ := connectionGetPortAndIP(udpConn)

		payloadBuffer := make([]byte, 50)
		payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
		payloadBufferWriter.WriteString("0")
		intPort, _ := strconv.Atoi(port)
		ackPacket := createMessage(ACK, int(m.sequenceNumber), ipAddress, intPort, payloadBufferWriter.Bytes())
		bAckPacket := convertMessageToBytes(ackPacket)

		log.Println("Data packet received frame #:" + strconv.Itoa(int(m.sequenceNumber)))

		// sending ACK
		_, err = udpConn.Write(bAckPacket.Bytes())
		log.Println("Ack packet sent for frame #:" + strconv.Itoa(int(m.sequenceNumber)))
		err = udpConn.SetReadDeadline(time.Now().Add(6 * time.Second))
	}
	return frames
}

func connectionGetPortAndIP(udpConn *net.UDPConn) (string, int) {
	split := strings.Split(udpConn.RemoteAddr().String(), ":")
	ipAddress := split[0]

	port, err := strconv.Atoi(split[1])
	checkError(&err)
	return ipAddress, port
}

func split(buf []byte, lim int) [][]byte {
	var chunk []byte
	chunks := make([][]byte, 0, len(buf)/lim+1)
	for len(buf) >= lim {
		chunk, buf = buf[:lim], buf[lim:]
		chunks = append(chunks, chunk)
	}
	if len(buf) > 0 {
		chunks = append(chunks, buf[:len(buf)])
	}
	return chunks
}

func handleHandshake(address string, udpConn *net.UDPConn, udpAddr *net.UDPAddr, port string, host string, bMessage bytes.Buffer) (*net.UDPConn, bytes.Buffer, string) {
	resolveUDPAddr := resolveAddress(address, udpConn)

	listenUDP, err := net.ListenUDP("udp", resolveUDPAddr)

	// Creating a deadline of 10 seconds
	// Start timer to send SYN again if did not receive SYN/ACK
	err = listenUDP.SetReadDeadline(time.Now().Add(10 * time.Second))
	checkError(&err)
	var resolveRemoteUDPAddr *net.UDPAddr
	for {
		buf := make([]byte, 1024)
		n, addr, err := listenUDP.ReadFromUDP(buf)
		if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
			// resend the SYN Request

			// Close listener
			err := listenUDP.Close()
			checkError(&err)

			// Initialize the 8080 Socket to resend the SYN packet
			udpConn, _ = initializeHandshake(udpAddr, port, host)
			resolveUDPAddr = resolveAddress(address, udpConn)

			// Reinitializing the ListenUDP
			listenUDP, err = net.ListenUDP("udp", resolveUDPAddr)
			checkError(&err)

			// Re-add the deadline
			err = listenUDP.SetReadDeadline(time.Now().Add(10 * time.Second))
			checkError(&err)
			continue
		}
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				continue
			}
		}

		// CHECK FOR SYN/ACK
		m := parseMessage(buf, n)

		log.Println("Received a " + strconv.Itoa(int(m.packetType)) + " packet as a response! from " + addr.String())

		if m.packetType != SYNACK {
			// ignore it and continue waiting
			continue
		}

		port = strconv.Itoa(int(m.peerPort))

		// Start Connection with socket handler from server side.
		err = listenUDP.Close()
		checkError(&err)

		// Reusing the local address previously used on the connection (Switching)
		resolveLocalUDPAddr, err := net.ResolveUDPAddr("udp", listenUDP.LocalAddr().String())
		checkError(&err)

		// Reusing the remote address previously used on the connection (Switching)
		resolveRemoteUDPAddr, err = net.ResolveUDPAddr("udp", addr.String())
		checkError(&err)

		udpConn, err = net.DialUDP("udp", resolveLocalUDPAddr, resolveRemoteUDPAddr)
		checkError(&err)

		payloadBuffer := make([]byte, 50)
		payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
		payloadBufferWriter.WriteString("0")
		// SEND ACK BACK TO CLIENT TO FINISH HANDSHAKE
		sPort, _ := strconv.Atoi(port)
		ackMessage := createMessage(ACK, 2, addr.IP.String(), sPort, payloadBufferWriter.Bytes())
		bMessage = convertMessageToBytes(ackMessage)

		// Send the ACK for the server
		_, err = udpConn.Write(bMessage.Bytes())
		checkError(&err)

		log.Println("Sending ACK to server on IP: " + udpConn.RemoteAddr().String())
		log.Println("Handshake Completed.")
		break
	}
	return udpConn, bMessage, port
}

func resolveAddress(address string, udpConn *net.UDPConn) *net.UDPAddr {
	// Start listening on port that was used in previous socket. (Switching)
	address = udpConn.LocalAddr().String()
	resolveUDPAddr, err := net.ResolveUDPAddr("udp", address)
	checkError(&err)
	return resolveUDPAddr
}

func creatingHttpRequest(opts *cli.Options) (*url.URL, string, string, string) {
	u := parseUrl(opts)
	host := u.Host
	port := u.Port()
	qry := ""
	if u.RawQuery != "" {
		qry += "?" + u.RawQuery
	}
	req := "" + opts.Method + " " + u.Path + qry + " HTTP/1.0\r\n"

	req = addHeaders(opts, req)

	if opts.Method == "POST" {
		if opts.Data != "" {
			parseInlineData(opts, &req)
		} else if opts.File != "" {
			parseFileData(opts, &req)
		} else {
			req += "\r\n"
		}
	} else {
		req += "\r\n"
	}
	return u, host, port, req
}

func initializeHandshake(udpAddr *net.UDPAddr, port string, host string) (*net.UDPConn, bytes.Buffer) {
	// Start Connection
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	checkError(&err)
	intPort, err := strconv.Atoi(port)
	checkError(&err)

	payloadBuffer := make([]byte, 50)
	payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
	payloadBufferWriter.WriteString("0")

	m := createMessage(SYN, 1, host, intPort, payloadBufferWriter.Bytes())
	bMessage := convertMessageToBytes(m)

	// Send the SYN Request
	log.Println("SYN packet sent to " + host + ":" + port)
	_, err = udpConn.Write(bMessage.Bytes())
	checkError(&err)

	// Close the 8080 port connection.
	err = udpConn.Close()
	checkError(&err)
	return udpConn, bMessage
}

func convertMessageToBytes(m encodedMessage) bytes.Buffer {
	// create the message buffer
	var bMessage bytes.Buffer
	bMessage.Write(m.packetType[:])
	bMessage.Write(m.sequenceNumber[:])
	bMessage.Write(m.peerAddress[:])
	bMessage.Write(m.peerPort[:])
	bMessage.Write(m.payload)
	return bMessage
}

func createMessage(packetType uint8, sequenceNumber int, host string, port int, payload []byte) encodedMessage {
	// Parse the address
	octets := strings.Split(host, ".")

	octet0, _ := strconv.Atoi(octets[0])
	octet1, _ := strconv.Atoi(octets[1])
	octet2, _ := strconv.Atoi(octets[2])
	octet3, _ := strconv.Atoi(octets[3])

	bAddress := [4]byte{byte(octet0), byte(octet1), byte(octet2), byte(octet3)}

	portBuffer := [2]byte{}
	binary.BigEndian.PutUint16(portBuffer[:], uint16(port))

	sequenceNumberBuffer := [4]byte{}
	binary.BigEndian.PutUint32(sequenceNumberBuffer[:], uint32(sequenceNumber))

	// Start Handshake
	// SYN MESSAGE
	m := encodedMessage{packetType: [1]byte{packetType}, sequenceNumber: sequenceNumberBuffer, peerAddress: bAddress, peerPort: portBuffer, payload: payload}
	return m
}

func parseMessage(buf []byte, n int) message {
	// Parse the address
	bAddressOctet0 := buf[5]
	bAddressOctet1 := buf[6]
	bAddressOctet2 := buf[7]
	bAddressOctet3 := buf[8]

	addressInt0 := strconv.Itoa(int(bAddressOctet0))
	addressInt1 := strconv.Itoa(int(bAddressOctet1))
	addressInt2 := strconv.Itoa(int(bAddressOctet2))
	addressInt3 := strconv.Itoa(int(bAddressOctet3))

	host := addressInt0 + "." + addressInt1 + "." + addressInt2 + "." + addressInt3

	port := binary.BigEndian.Uint16(buf[9:11])

	m := message{}
	m.packetType = buf[0]
	m.sequenceNumber = binary.BigEndian.Uint32(buf[1:5])
	m.peerAddress = host
	m.peerPort = port
	m.payload = string(buf[11:n])
	return m
}

func ParseResponse(payload string, res *Response, resString *string) {
	r := strings.NewReader(payload)
	scnr := bufio.NewScanner(r)
	// Scan status line
	if !scnr.Scan() {
		fmt.Println("No response.")
		os.Exit(1)
	}

	line := scnr.Text()
	split := strings.Split(line, " ")

	proto := split[0]
	statusCode := split[1]

	res.Proto = proto
	res.StatusCode, _ = strconv.Atoi(statusCode)
	res.Headers = make(map[string]string, 10)
	*resString = line + "\n"
	// Read the Headers
	for scnr.Scan() {
		line := scnr.Text()
		// When we see the blank line, Headers are done
		if line == "" {
			*resString += "\n"
			break
		}
		*resString += line + "\n"
		index := strings.Index(line, ":")
		key := line[:index]
		value := line[index+1:]
		res.Headers[key] = value
	}

	// print the Body
	for scnr.Scan() {
		line := scnr.Text()
		*resString += line + "\n"
		res.Body = res.Body + line + "\n"
	}
}

func parseInlineData(opts *cli.Options, req *string) {
	if _, ok := opts.Header["Content-Length"]; !ok {
		length := strconv.Itoa(len(opts.Data))
		*req += "Content-Length: " + length + "\r\n"
		opts.Header["Content-Length"] = length
	}
	*req += "\r\n" + opts.Data
}

func parseFileData(opts *cli.Options, req *string) {
	if _, ok := opts.Header["Content-Length"]; !ok {
		data, _ := ioutil.ReadFile(opts.File)
		length := strconv.Itoa(len(data))
		*req += "Content-Length: " + length + "\r\n"
		opts.Header["Content-Length"] = length
		*req += "\r\n" + string(data)
	}
}

func addHeaders(opts *cli.Options, req string) string {
	if len(opts.Header) > 0 {
		for key, val := range opts.Header {
			req = req + key + ": " + val + "\r\n"
		}
	}
	//req += "Connection: close\r\n" + "\r\n"
	return req
}

func parseProtocol(u *url.URL, port string) string {
	if u.Port() == "" && u.Scheme == "" {
		MissingProtocolErrorMessage()
	}

	if u.Port() == "" {
		UnsupportedProtocolErrorMessage()
	}
	return port
}

func UnsupportedProtocolErrorMessage() {
	fmt.Println("Unsupported Protocol")
	os.Exit(1)
}

func MissingProtocolErrorMessage() {
	fmt.Println("Missing Protocol")
	os.Exit(1)
}

func parseUrl(opts *cli.Options) *url.URL {
	u, err := url.Parse(opts.Url)
	if err != nil {
		fmt.Println("Url Parsing Error.")
		os.Exit(1)
	}
	return u
}

func checkError(e *error) {
	if *e != nil {
		fmt.Println("Internal Socket Error.")
		os.Exit(1)
	}
}
