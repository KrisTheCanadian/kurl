package cli

import (
	"flag"
	"fmt"
	"os"
)

type options struct {
	method  int
	verbose bool
	query   string
	header  string
	data    string
	file    string
	url     string
}

const (
	get int = iota
	post
)

func Parse() {
	opts := options{}
	getFlag, getVerboseFlag, getHeaderFlag, postFlag, postVerboseFlag, postHeaderFlag, postDataFlag, postFileFlag := initFlags()

	if len(os.Args) < 2 {
		fmt.Println("expected 'get' or 'post' commands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "get":
		setGetOptions(opts, getFlag, getVerboseFlag, getHeaderFlag)
	case "post":
		setPostOptions(opts, postFlag, postVerboseFlag, postHeaderFlag, postFileFlag, postDataFlag)

	default:
		fmt.Println("expected 'get' or 'post' commands")
		os.Exit(1)
	}
}

func setGetOptions(opts options, getFlag *flag.FlagSet, getVerboseFlag *bool, getHeaderFlag *string) {
	opts.method = get
	getFlag.Parse(os.Args[2:])
	opts.verbose = *getVerboseFlag
	opts.header = *getHeaderFlag

	if getFlag.Args() == nil {
		fmt.Println("expected a url")
		os.Exit(1)
	}

	opts.url = flag.Args()[0]
	// TODO Remove
	fmt.Println(opts)
}

func setPostOptions(opts options, postFlag *flag.FlagSet, postVerboseFlag *bool, postHeaderFlag *string, postFileFlag *string, postDataFlag *string) {
	opts.method = post
	postFlag.Parse(os.Args[2:])
	opts.verbose = *postVerboseFlag
	opts.header = *postHeaderFlag

	if *postFileFlag != "" && *postDataFlag != "" {
		fmt.Println("Either [-d] or [-f] can be used but not both.")
		os.Exit(1)
	}

	opts.data = *postDataFlag
	opts.file = *postFileFlag

	if postFlag.Args() == nil {
		fmt.Println("expected a url")
		os.Exit(1)
	}

	opts.url = postFlag.Args()[0]
	// TODO Remove
	fmt.Println(opts)
}

func initFlags() (*flag.FlagSet, *bool, *string, *flag.FlagSet, *bool, *string, *string, *string) {
	getFlag := flag.NewFlagSet("get", flag.ExitOnError)
	getVerboseFlag := getFlag.Bool("v", false, "Prints the detail of the response such as protocol, status,\nand headers.")
	getHeaderFlag := getFlag.String("h", "", "header")

	postFlag := flag.NewFlagSet("post", flag.ExitOnError)
	postVerboseFlag := postFlag.Bool("v", false, "Prints the detail of the response such as protocol, status,\nand headers.")
	postHeaderFlag := postFlag.String("h", "", "header")
	postDataFlag := postFlag.String("d", "", "Associates an inline data to the body HTTP POST request.")
	postFileFlag := postFlag.String("f", "", "Associates the content of a file to the body HTTP POST")
	return getFlag, getVerboseFlag, getHeaderFlag, postFlag, postVerboseFlag, postHeaderFlag, postDataFlag, postFileFlag
}
