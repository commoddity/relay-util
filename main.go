package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/commoddity/relay-util/env"
	"github.com/commoddity/relay-util/log"
	"github.com/commoddity/relay-util/relay"
	"github.com/commoddity/relay-util/setup"
	"github.com/joho/godotenv"
	"github.com/spf13/pflag"
)

// init is a special function that is called before the main function
// and sets up the flags and usage information for the program.
func init() {
	_ = godotenv.Load(env.EnvPath)

	// Override the default help flag
	pflag.BoolP("help", "h", false, "Display help information")

	// Customize the usage function to provide detailed flag descriptions
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "This program sends relays to a specified blockchain and logs the results. It supports various flags to control its behavior.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		pflag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample command:\n")
		fmt.Fprintf(os.Stderr, "  %s \\\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "    -c=solana-mainnet-custom \\\n")
		fmt.Fprintf(os.Stderr, "    -e=staging \\\n")
		fmt.Fprintf(os.Stderr, "    -p=enterprise \\\n")
		fmt.Fprintf(os.Stderr, "    -r='{\"jsonrpc\": \"2.0\", \"id\": 1, \"method\": \"getAccountInfo\", \"params\": [\"11111111111111111111111111111111\"]}' \\\n")
		fmt.Fprintf(os.Stderr, "    -x=3000 \\\n")
		fmt.Fprintf(os.Stderr, "    -d=10 \\\n")
		fmt.Fprintf(os.Stderr, "    -g=500\n")
		fmt.Fprintf(os.Stderr, "    -t=20\n")
	}
}

func main() {

	/* Flag Parsing */

	var chain, envStr, planType, request, overrideURL, authorization string
	var executions, goroutines, delay, timeout int
	var local, successBodies bool

	// Required flags
	pflag.StringVarP(&chain, "chain", "c", "", "The chain alias to which the relays will be sent.")
	pflag.StringVarP(&envStr, "env", "e", "", "The environment where the relays will be sent. Valid values are 'production' or 'staging'.")
	pflag.StringVarP(&planType, "planType", "p", "", "The plan type under which the relays are sent. Valid values are 'starter' or 'enterprise'.")
	pflag.IntVarP(&executions, "executions", "x", 1, "The total number of relays to execute. This defines how many times the relay will be sent.")
	pflag.StringVarP(&request, "request", "r", "", "The JSON RPC request body that will be sent as the relay. Must be a valid JSON string.")
	pflag.BoolVarP(&local, "local", "l", false, "A flag to indicate if the relays should be sent to a local environment. Useful for testing locally.")
	pflag.BoolVarP(&successBodies, "success-bodies", "s", false, "A flag that, when set, will cause the bodies of successful relay responses to be displayed in the log output.")
	pflag.StringVarP(&overrideURL, "override-url", "o", "", "A custom URL to override the default endpoint. This allows you to specify a different URL for sending relays.")
	pflag.StringVarP(&authorization, "authorization", "a", "", "Override the Authorization header with a custom value.")
	pflag.IntVarP(&goroutines, "goroutines", "g", 0, "The level of concurrency for sending relays. This defines how many goroutines will be used to send relays in parallel.")
	pflag.IntVarP(&delay, "delay", "d", 10, "The delay between individual relay requests, measured in milliseconds. This helps to control the rate at which relays are sent.")
	pflag.IntVarP(&timeout, "timeout", "t", 20, "The timeout for individual relay requests, measured in seconds. [default: 20]")

	pflag.Parse()

	// Check if help was requested
	helpFlag := pflag.Lookup("help")
	if helpFlag != nil && helpFlag.Value.String() == "true" {
		pflag.Usage()
		return // Exit gracefully without calling os.Exit
	}

	// Check if the .env file exists, if not, prompt the user to create it
	setup.Start()

	if chain == "" {
		fmt.Println("🚫 Missing required flag: -c, --chain for chain. Use --help for more information.")
		os.Exit(1)
	}
	if envStr == "" {
		fmt.Println("🚫 Missing required flag: -e, --env for environment. Use --help for more information.")
		os.Exit(1)
	}
	if planType == "" {
		fmt.Println("🚫 Missing required flag: -p, --planType for planType. Use --help for more information.")
		os.Exit(1)
	}
	if executions == 0 {
		fmt.Println("🚫 Missing required flag: -x, --executions for executions. Use --help for more information.")
		os.Exit(1)
	}
	if request == "" {
		fmt.Println("🚫 Missing required flag: -r, --request for request. Use --help for more information.")
		os.Exit(1)
	}
	if envStr != "production" && envStr != "staging" {
		fmt.Println("🚫 Environment must be either 'production' or 'staging'. Use --help for more information.")
		os.Exit(1)
	}
	if planType != "starter" && planType != "enterprise" {
		fmt.Println("🚫 Plan type must be either 'starter' or 'enterprise'. Use --help for more information.")
		os.Exit(1)
	}
	if planType == "enterprise" && goroutines == 0 {
		fmt.Println("🚫 For enterprise plans, the -g, --goroutines flag is required. Use --help for more information.")
		os.Exit(1)
	}
	if planType == "enterprise" && delay == 0 {
		fmt.Println("🚫 For enterprise plans, the -d, --delay flag is required. Use --help for more information.")
		os.Exit(1)
	}
	if _, err := strconv.Atoi(strconv.Itoa(executions)); err != nil {
		fmt.Println("🚫 Executions must be a valid integer. Use --help for more information.")
		os.Exit(1)
	}
	var jsonRaw json.RawMessage
	if err := json.Unmarshal([]byte(request), &jsonRaw); err != nil {
		fmt.Println("🚫 Request must be a valid JSON RPC body. Use --help for more information.")
		os.Exit(1)
	}
	if planType == "starter" && goroutines > 30 {
		fmt.Println("✨ Starter plans are locked at 30 requests per second to avoid hitting the throughput limit.")
	}

	/* Relay Util Init */

	config := relay.Config{
		Request:       request,
		Chain:         chain,
		Env:           env.EnvType(envStr),
		Local:         local,
		SuccessBodies: successBodies,
		PlanType:      env.PlanType(planType),
		Executions:    executions,
		Goroutines:    goroutines,
		Delay:         time.Duration(delay) * time.Millisecond,
		Timeout:       time.Duration(timeout) * time.Second,
		OverrideURL:   overrideURL,
		Authorization: authorization,
	}

	relayUtil := relay.NewRelayUtil(config)

	/* Send Relays */

	log.PrintConfig(relayUtil)

	relayUtil.SendRelays()

	log.LogResults(relayUtil)
}
