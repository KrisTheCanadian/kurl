package main

import (
	"github.com/kristhecanadian/kurl/cli"
	"log"
)

func main() {
	cli.Parse()
	//con, err := net.Dial("tcp", "google.ca:80")
	//checkError(&err)
	//
	//req := "HEAD / HTTP/1.0\r\n\r\n"
	//
	//_, err = con.Write([]byte(req))
	//checkError(&err)
	//
	//res, err := ioutil.ReadAll(con)
	//checkError(&err)
	//
	//fmt.Println(string(res))
}

func checkError(err *error) {
	if *err != nil {
		log.Fatal(*err)
	}
}
