package req

import (
	"bufio"
	"fmt"
	"github.com/kristhecanadian/kurl/cli"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	http int = 80
)

type Response struct {
	Proto      string
	StatusCode int
	Headers    map[string]string
	Body       string
}

func Request(opts *cli.Options) (res Response, resString string) {
	u := parseUrl(opts)
	host := u.Host
	port := u.Port()
	qry := ""
	if u.RawQuery != "" {
		qry += "?" + u.RawQuery
	}
	req := "" + opts.Method + " " + u.Path + qry + " HTTP/1.1\r\n"

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

	con, err := net.Dial("tcp", ""+host+":"+port)
	checkError(&err)

	writeRequest(err, con, req)

	scnr := bufio.NewScanner(con)

	res = Response{}
	ParseResponse(scnr, &res, &resString)

	return res, resString
}

func ParseResponse(scnr *bufio.Scanner, res *Response, resString *string) {
	// Scan status line
	if !scnr.Scan() {
		panic("No status line!")
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
