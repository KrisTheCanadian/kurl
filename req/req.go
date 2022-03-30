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
	"strconv"
	"strings"
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
	port = parseProtocol(u, port)

	address := "" + host + ":" + port

	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		fmt.Print("Address is not resolved.")
		return
	}

	// Start Connection
	con, err := net.DialUDP("udp", nil, udpAddr)
	checkError(&err)

	// Parse the address
	octets := strings.Split(host, ".")

	octet0, _ := strconv.Atoi(octets[0])
	octet1, _ := strconv.Atoi(octets[1])
	octet2, _ := strconv.Atoi(octets[2])
	octet3, _ := strconv.Atoi(octets[3])

	bAddress := [4]byte{byte(octet0), byte(octet1), byte(octet2), byte(octet3)}

	fmt.Printf("%s has 4-byte representation of %bAddress\n", host, bAddress)

	portBuffer := [2]byte{}
	binary.LittleEndian.PutUint16(portBuffer[:], 8080)

	sequenceNumberBuffer := [4]byte{}
	binary.BigEndian.PutUint32(sequenceNumberBuffer[:], 0)

	// Start Handshake
	// SYN MESSAGE
	m := encodedMessage{packetType: [1]byte{SYN}, sequenceNumber: sequenceNumberBuffer, peerAddress: bAddress, peerPort: portBuffer, payload: []byte("0")}
	fmt.Println(m)

	// create the message buffer
	var bMessage bytes.Buffer
	bMessage.Write(m.packetType[:])
	bMessage.Write(m.sequenceNumber[:])
	bMessage.Write(m.peerAddress[:])
	bMessage.Write(m.peerPort[:])
	bMessage.Write(m.payload)

	// Send the SYN Request
	_, err = con.Write(bMessage.Bytes())
	// Start timer to send SYN again if did not receive SYN/ACK
	// WAIT FOR SYN/ACK
	// OR TIMER
	// SELECT IN GO -> Switch statement with channels . Channel with time, Channel for the SYN/ACK

	if err != nil {
		log.Fatal(err)
	}

	// HTTP STUFF
	writeRequest(err, con, req)

	res = Response{}

	ParseResponse(con, &res, &resString)

	return res, resString
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

func writeRequest(err error, con net.Conn, req string) {
	_, err = con.Write([]byte(req))
	checkError(&err)
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
