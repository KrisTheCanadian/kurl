package req

import (
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

func Request(opts *cli.Options) {
	u := parseUrl(opts)
	host := u.Host
	port := u.Port()
	req := "" + opts.Method + " / HTTP/1.0\r\n"

	req = parseHeader(opts, req)

	port = parseProtocol(u, port)

	con, err := net.Dial("tcp", ""+host+":"+port)
	checkError(&err)

	_, err = con.Write([]byte(req))
	checkError(&err)

	res, err := ioutil.ReadAll(con)
	checkError(&err)

	fmt.Println(string(res))
}

func parseHeader(opts *cli.Options, req string) string {
	if len(opts.Header) > 0 {
		headers := strings.Fields(opts.Header)

		for i := 0; i < len(headers); i++ {
			index := strings.Index(headers[i], ":") + 1
			headers[i] = headers[i][:index] + " " + headers[i][index:] + "\\r\\n"
			req = req + headers[i]
		}
		lastStringIndex := len(headers) - 1
		headers[lastStringIndex] = headers[lastStringIndex] + "\r\n"
	} else {
		req = req + "\r\n"
	}
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
