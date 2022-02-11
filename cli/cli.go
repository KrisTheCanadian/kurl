package cli

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
)

type Options struct {
	Method  string
	Verbose bool
	Query   string
	Header  map[string]string
	Data    string
	File    string
	Url     string
}

type headerFlags map[string]string

func (h headerFlags) String() string {
	return ""
}

func (h headerFlags) Set(s string) error {
	if s == "" {
		return nil
	}
	index := strings.Index(s, ":")
	key := s[:index]
	value := s[index+1:]
	if key == "" || value == "" {
		fmt.Println(`format should be "k:v"`)
		os.Exit(1)
	}
	h[key] = value
	return nil
}

func Parse() *Options {
	opts := Options{}
	getFlag, getVerboseFlag, headerFlagValue, postFlag, postVerboseFlag, postDataFlag, postFileFlag := initFlags()

	if len(os.Args) < 2 {
		UsageErrorMessage()
	}

	switch os.Args[1] {
	case "get":
		if len(os.Args) < 3 {
			getErrorMessage()
		}
		setGetOptions(&opts, getFlag, getVerboseFlag, &headerFlagValue)
	case "post":
		if len(os.Args) < 3 {
			postErrorMessage()
		}
		setPostOptions(&opts, postFlag, postVerboseFlag, &headerFlagValue, postFileFlag, postDataFlag)
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

func setGetOptions(opts *Options, getFlag *flag.FlagSet, getVerboseFlag *bool, headerFlagValue *headerFlags) {
	opts.Method = "GET"
	getFlag.Parse(os.Args[2:])
	opts.Verbose = *getVerboseFlag
	opts.Header = *headerFlagValue

	if len(getFlag.Args()) < 0 || getFlag.Args()[0] == "" {
		UrlErrorMessage()
	}
	opts.Url = getFlag.Args()[0]
	ValidateUrl(opts)
}

func setPostOptions(opts *Options, postFlag *flag.FlagSet, postVerboseFlag *bool, headerFlagValue *headerFlags, postFileFlag *string, postDataFlag *string) {
	opts.Method = "POST"
	postFlag.Parse(os.Args[2:])
	opts.Verbose = *postVerboseFlag
	opts.Header = *headerFlagValue

	if *postFileFlag != "" && *postDataFlag != "" {
		fmt.Println("Either [-d] or [-f] can be used but not both.")
		os.Exit(1)
	}

	opts.Data = *postDataFlag
	opts.File = *postFileFlag

	if len(postFlag.Args()) < 0 || postFlag.Args()[0] == "" {
		UrlErrorMessage()
	}

	opts.Url = postFlag.Args()[0]
	ValidateUrl(opts)
}

func initFlags() (*flag.FlagSet, *bool, headerFlags, *flag.FlagSet, *bool, *string, *string) {
	headerFlagValue := make(headerFlags, 10)
	getFlag := flag.NewFlagSet("get", flag.ExitOnError)
	getVerboseFlag := getFlag.Bool("v", false, "Prints the detail of the response such as protocol, status,\nand headers.")
	getFlag.Var(&headerFlagValue, "h", "header")

	postFlag := flag.NewFlagSet("post", flag.ExitOnError)
	postVerboseFlag := postFlag.Bool("v", false, "Prints the detail of the response such as protocol, status,\nand headers.")
	postFlag.Var(&headerFlagValue, "h", "header")
	postDataFlag := postFlag.String("d", "", "Associates an inline Data to the body HTTP POST request.")
	postFileFlag := postFlag.String("f", "", "Associates the content of a File to the body HTTP POST")
	return getFlag, getVerboseFlag, headerFlagValue, postFlag, postVerboseFlag, postDataFlag, postFileFlag
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
	fmt.Println("usage: httpc post [-v] [-h key:value] [-d inline-Data] [-f File] URL")
	os.Exit(1)
}
