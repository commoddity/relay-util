package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/commoddity/relay-util/v2/log"
	"github.com/commoddity/relay-util/v2/relay"
	"github.com/spf13/pflag"
)

// init is a special function that is called before the main function
// and sets up the flags and usage information for the program.
func init() {
	// Override the default help flag
	pflag.BoolP("help", "h", false, "Display help information")

	// Customize the usage function to provide detailed flag descriptions
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "This program sends relays to a specified service and logs the results. It supports various flags to control its behavior.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		pflag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample command:\n")
		fmt.Fprintf(os.Stderr, "  %s \\\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "    -u=https://path.rpc.grove.city/v1 \\\n")
		fmt.Fprintf(os.Stderr, "    -H=\"target-service-id: F00C\" \\\n")
		fmt.Fprintf(os.Stderr, "    -d='{\"jsonrpc\": \"2.0\", \"id\": 1, \"method\": \"eth_blockNumber\", \"params\": []}' \\\n")
		fmt.Fprintf(os.Stderr, "    -x=3000 \\\n")
		fmt.Fprintf(os.Stderr, "    -w=10 \\\n")
		fmt.Fprintf(os.Stderr, "    -g=500 \\\n")
		fmt.Fprintf(os.Stderr, "    -t=20 \\\n")
		fmt.Fprintf(os.Stderr, "    -H=\"Authorization: api_key_123\" \\\n")
		fmt.Fprintf(os.Stderr, "    -H=\"Custom-Header: value\"\n")
	}
}

func main() {
	/* Flag Parsing */
	var data, url string
	var executions, goroutines, wait, timeout int
	var successBodies bool
	var headers []string

	// Required flags
	pflag.StringVarP(&url, "url", "u", "", "[REQUIRED] The URL to send the requests to.")

	// Optional flags
	pflag.StringVarP(&data, "data", "d", "", "[OPTIONAL] The request body that will be sent as the relay. Must be a valid JSON string.")
	pflag.StringSliceVarP(&headers, "headers", "H", nil, "[OPTIONAL] Custom headers to include in the relay request, specified as -H \"Header-Name: value\". Can be used multiple times.")
	pflag.IntVarP(&executions, "executions", "x", 1, "[OPTIONAL] The total number of relays to execute. This defines how many times the relay will be sent.")
	pflag.BoolVarP(&successBodies, "success-bodies", "b", false, "[OPTIONAL] A flag that, when set, will cause the bodies of successful relay responses to be displayed in the log output.")
	pflag.IntVarP(&goroutines, "goroutines", "g", 5, "[OPTIONAL] The level of concurrency for sending relays. This defines how many goroutines will be used to send relays in parallel.")
	pflag.IntVarP(&wait, "wait", "w", 10, "[OPTIONAL] The delay between individual relay requests, measured in milliseconds. This helps to control the rate at which relays are sent.")
	pflag.IntVarP(&timeout, "timeout", "t", 20, "[OPTIONAL] The timeout for individual relay requests, measured in seconds.")

	pflag.Parse()

	// Convert headers from []string to http.Header
	headerMap := make(http.Header)
	for _, h := range headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			headerMap.Add(key, value)
		} else {
			fmt.Println("🚫 Invalid header format. Use -H \"Header-Name: value\". Use --help for more information.")
			os.Exit(1)
		}
	}

	// Check if help was requested
	helpFlag := pflag.Lookup("help")
	if helpFlag != nil && helpFlag.Value.String() == "true" {
		pflag.Usage()
		return // Exit gracefully without calling os.Exit
	}

	if url == "" {
		fmt.Println("🚫 Missing required flag: -u, --url for URL. Use --help for more information.")
		os.Exit(1)
	}
	if executions == 0 {
		fmt.Println("🚫 Executions must be greater than 0. Use --help for more information.")
		os.Exit(1)
	}
	if _, err := strconv.Atoi(strconv.Itoa(executions)); err != nil {
		fmt.Println("🚫 Executions must be a valid integer. Use --help for more information.")
		os.Exit(1)
	}

	/* Relay Util Init */
	relayUtil := relay.NewRelayUtil(relay.Config{
		URL:           url,
		Body:          []byte(data),
		Headers:       headerMap,
		Executions:    executions,
		Goroutines:    goroutines,
		Wait:          time.Duration(wait) * time.Millisecond,
		Timeout:       time.Duration(timeout) * time.Second,
		SuccessBodies: successBodies,
	})

	/* Send Relays */

	log.PrintConfig(relayUtil)

	relayUtil.SendRelays()

	log.LogResults(relayUtil)
}
