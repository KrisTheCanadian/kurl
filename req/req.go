package req

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/kristhecanadian/kurl/cli"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	http int = 8080
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

	address := "" + host + ":" + port

	udpAddr, err := net.ResolveUDPAddr("udp", address)
	checkError(&err)

	udpConn, bMessage := initializeHandshake(udpAddr, port, host)
	checkError(&err)

	// Start listening on port that was used in previous socket. (Switching)
	address = udpConn.LocalAddr().String()
	resolveUDPAddr, err := net.ResolveUDPAddr("udp", address)
	checkError(&err)

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
			initializeHandshake(udpAddr, port, host)

			// Reinitializing the ListenUDP
			listenUDP, err = net.ListenUDP("udp", resolveUDPAddr)
			checkError(&err)

			// Re-add the deadline
			err = listenUDP.SetReadDeadline(time.Now().Add(15 * time.Second))
			checkError(&err)
			continue
		}
		if err != nil {
			if e, ok := err.(net.Error); !ok || !e.Timeout() {
				continue
			}
		}
		fmt.Println(buf[:n])
		fmt.Println("Received a packet as a response! from " + addr.String())

		// CHECK FOR SYN/ACK
		m := parseMessage(buf, err, n)
		if m.packetType != SYNACK {
			// ignore it and continue waiting
			continue
		}

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

		payloadBuffer := make([]byte, 1024)
		payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
		payloadBufferWriter.WriteString("0")
		// SEND ACK BACK TO CLIENT TO FINISH HANDSHAKE
		ackMessage := createMessage(ACK, 2, addr.IP.String(), addr.Port, payloadBufferWriter.Bytes())
		bMessage = convertMessageToBytes(ackMessage)

		// Send the ACK for the server
		_, err = udpConn.Write(bMessage.Bytes())
		fmt.Println(bMessage.Bytes())
		checkError(&err)

		fmt.Println("Sending ACK to server on IP: " + udpConn.RemoteAddr().String())
		fmt.Println("Handshake Completed.")
		break
	}
	// TODO Handle ACK DROP -> Keep listening on a different channel?
	// TEMP FIX? IF RECEIVE ANOTHER SYN/ACK JUST ACK IT.
	// Create Frames and Sequence Numbers for HTTP Request
	// TODO SPLIT THE REQUEST INTO MANY FRAMES IF TOO LARGE
	frames := make(map[int]encodedMessage)
	ackedFrames := make(map[int]bool)

	breq := []byte(req)
	requestLength := len(breq)
	fmt.Println(requestLength)

	m := createMessage(DATA, 10, resolveRemoteUDPAddr.IP.String(), resolveRemoteUDPAddr.Port, breq)
	frames[10] = m
	bMessage = convertMessageToBytes(m)
	// SEND THE MESSAGE FRAMES
	_, err = udpConn.Write(bMessage.Bytes())
	
	res = Response{}

	ParseResponse(udpConn, &res, &resString)

	return res, resString

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

	payloadBuffer := make([]byte, 1024)
	payloadBufferWriter := bytes.NewBuffer(payloadBuffer)
	payloadBufferWriter.WriteString("0")

	m := createMessage(SYN, 1, host, intPort, payloadBufferWriter.Bytes())
	fmt.Println(m)

	bMessage := convertMessageToBytes(m)

	// Send the SYN Request
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
	binary.LittleEndian.PutUint16(portBuffer[:], uint16(port))

	sequenceNumberBuffer := [4]byte{}
	binary.BigEndian.PutUint32(sequenceNumberBuffer[:], uint32(sequenceNumber))

	// Start Handshake
	// SYN MESSAGE
	m := encodedMessage{packetType: [1]byte{packetType}, sequenceNumber: sequenceNumberBuffer, peerAddress: bAddress, peerPort: portBuffer, payload: payload}
	return m
}

func parseMessage(buf []byte, err error, n int) message {
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

	port := binary.LittleEndian.Uint16(buf[9:11])

	m := message{}
	m.packetType = buf[0]
	m.sequenceNumber = binary.BigEndian.Uint32(buf[1:5])
	m.peerAddress = host
	m.peerPort = port
	m.payload = string(buf[11:n])
	return m
}

func ParseResponse(con net.Conn, res *Response, resString *string) {
	defer con.Close()
	scnr := bufio.NewScanner(con)
	// Scan status line
	if !scnr.Scan() {
		fmt.Print("No response.")
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
		switch u.Scheme {
		case "http":
			port = strconv.Itoa(http)
		default:
			UnsupportedProtocolErrorMessage()
		}
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
