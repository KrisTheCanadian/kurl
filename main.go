package main

import (
	"fmt"
	"github.com/kristhecanadian/kurl/cli"
	"github.com/kristhecanadian/kurl/req"
	"log"
)

func main() {
	opts := cli.Parse()
	fmt.Println(opts)
	fmt.Println("done!")
	req.Request(opts)
}

func checkError(err *error) {
	if *err != nil {
		log.Fatal(*err)
	}
}
