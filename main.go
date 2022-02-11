package main

import (
	"fmt"
	"github.com/kristhecanadian/kurl/cli"
	"github.com/kristhecanadian/kurl/req"
)

func main() {
	opts := cli.Parse()
	resObj, res := req.Request(opts)
	if opts.Verbose {
		fmt.Println(res)
	} else {
		fmt.Println(resObj.Body)
	}

}
