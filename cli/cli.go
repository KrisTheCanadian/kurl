package cli

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
)

type Options struct {
	Method  string
	verbose bool
	query   string
	Header  string
	data    string
	file    string
	Url     string
}

func Parse() *Options {
	opts := Options{}
	getFlag, getVerboseFlag, getHeaderFlag, postFlag, postVerboseFlag, postHeaderFlag, postDataFlag, postFileFlag := initFlags()

	if len(os.Args) < 2 {
		UsageErrorMessage()
	}

	switch os.Args[1] {
	case "get":
		if len(os.Args) < 3 {
			getErrorMessage()
		}
		setGetOptions(&opts, getFlag, getVerboseFlag, getHeaderFlag)
	case "post":
		if len(os.Args) < 3 {
			postErrorMessage()
		}
		setPostOptions(&opts, postFlag, postVerboseFlag, postHeaderFlag, postFileFlag, postDataFlag)
	case "help":
		if len(os.Args) < 3 {
			UsageErrorMessage()
		}

		if os.Args[2] == "get" {
			getErrorMessage()
		}
		if os.Args[2] == "post" {
			postErrorMessage()
		}

	default:
		ArgumentErrorMessage()
	}

	return &opts
}

func setGetOptions(opts *Options, getFlag *flag.FlagSet, getVerboseFlag *bool, getHeaderFlag *string) {
	opts.Method = "GET"
	getFlag.Parse(os.Args[2:])
	opts.verbose = *getVerboseFlag
	opts.Header = *getHeaderFlag

	if len(getFlag.Args()) < 0 || getFlag.Args()[0] == "" {
		UrlErrorMessage()
	}
	opts.Url = getFlag.Args()[0]
	ValidateUrl(opts)
}

func setPostOptions(opts *Options, postFlag *flag.FlagSet, postVerboseFlag *bool, postHeaderFlag *string, postFileFlag *string, postDataFlag *string) {
	opts.Method = "POST"
	postFlag.Parse(os.Args[2:])
	opts.verbose = *postVerboseFlag
	opts.Header = *postHeaderFlag

	if *postFileFlag != "" && *postDataFlag != "" {
		fmt.Println("Either [-d] or [-f] can be used but not both.")
		os.Exit(1)
	}

	opts.data = *postDataFlag
	opts.file = *postFileFlag

	if len(postFlag.Args()) < 0 || postFlag.Args()[0] == "" {
		UrlErrorMessage()
	}

	opts.Url = postFlag.Args()[0]
	ValidateUrl(opts)
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

func ValidateUrl(opts *Options) {
	_, err := url.ParseRequestURI(opts.Url)
	if err != nil {
		log.Printf("Invalid Url")
		os.Exit(1)
	}
}

func UrlErrorMessage() {
	fmt.Println("expected a url")
	os.Exit(1)
}

func ArgumentErrorMessage() {
	fmt.Println("expected 'get', 'post' or 'help' commands")
	os.Exit(1)
}

func UsageErrorMessage() {
	fmt.Println("httpc is a curl-like application but supports HTTP protocol only." +
		"\nUsage:" +
		"\nhttpc command [arguments]" +
		"\nThe commands are:" +
		"\nget executes a HTTP GET request and prints the response." +
		"\npost executes a HTTP POST request and prints the response." +
		"\nhelp prints this screen.\nUse \"httpc help [command]\" for more information about a command")
	os.Exit(1)
}

func getErrorMessage() {
	fmt.Println("usage: httpc get [-v] [-h key:value] URL")
	os.Exit(1)
}

func postErrorMessage() {
	fmt.Println("usage: httpc post [-v] [-h key:value] [-d inline-data] [-f file] URL")
	os.Exit(1)
}
