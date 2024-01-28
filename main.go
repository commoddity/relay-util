package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/commoddity/relay-util/env"
	"github.com/commoddity/relay-util/relay"
	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

func init() {
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
	}
}

func main() {
	// Check if the .env file exists, if not, prompt the user to create it
	checkEnvFile()

	var chain, env, planType, request, overrideURL string
	var executions, goroutines, delay int
	var local, successBodies bool

	pflag.StringVarP(&chain, "chain", "c", "", "The chain alias to which the relays will be sent.")
	pflag.StringVarP(&env, "env", "e", "", "The environment where the relays will be sent. Valid values are 'production' or 'staging'.")
	pflag.StringVarP(&planType, "planType", "p", "", "The plan type under which the relays are sent. Valid values are 'starter' or 'enterprise'.")
	pflag.IntVarP(&executions, "executions", "x", 1, "The total number of relays to execute. This defines how many times the relay will be sent.")
	pflag.StringVarP(&request, "request", "r", "", "The JSON RPC request body that will be sent as the relay. Must be a valid JSON string.")
	pflag.BoolVarP(&local, "local", "l", false, "A flag to indicate if the relays should be sent to a local environment. Useful for testing locally.")
	pflag.BoolVarP(&successBodies, "success-bodies", "s", false, "A flag that, when set, will cause the bodies of successful relay responses to be displayed in the log output.")
	pflag.StringVarP(&overrideURL, "override-url", "o", "", "A custom URL to override the default endpoint. This allows you to specify a different URL for sending relays.")
	pflag.IntVarP(&goroutines, "goroutines", "g", 0, "The level of concurrency for sending relays. This defines how many goroutines will be used to send relays in parallel.")
	pflag.IntVarP(&delay, "delay", "d", 10, "The delay between individual relay requests, measured in milliseconds. This helps to control the rate at which relays are sent.")

	pflag.Parse()

	// Check if help was requested
	helpFlag := pflag.Lookup("help")
	if helpFlag != nil && helpFlag.Value.String() == "true" {
		pflag.Usage()
		return // Exit gracefully without calling os.Exit
	}

	if chain == "" || env == "" || planType == "" || executions == 0 || request == "" {
		fmt.Println("All flags are required. They are -c for chain, -e for environment, -p for planType, -x for executions, -r for request.")
		os.Exit(1)
	}

	if env != "production" && env != "staging" {
		fmt.Println("Environment must be either 'production' or 'staging'")
		os.Exit(1)
	}

	if planType != "starter" && planType != "enterprise" {
		fmt.Println("Plan type must be either 'starter' or 'enterprise'")
		os.Exit(1)
	}
	if planType == "starter" && goroutines > 30 {
		fmt.Println("Starter plans are locked at 30 requests per second to avoid hitting the throughput limit.")
	}
	if planType == "enterprise" && goroutines == 0 {
		fmt.Println("For enterprise plans, the --goroutines, -g flag is required.")
		os.Exit(1)
	}

	if _, err := strconv.Atoi(strconv.Itoa(executions)); err != nil {
		fmt.Println("executions must be a valid integer")
		os.Exit(1)
	}

	var jsonRaw json.RawMessage
	if err := json.Unmarshal([]byte(request), &jsonRaw); err != nil {
		fmt.Println("request must be a valid JSON RPC body")
		os.Exit(1)
	}

	appIDs := gatherAppIDs()
	config := relayConfig{
		appIDs:        appIDs,
		request:       request,
		chain:         chain,
		env:           envType(env),
		local:         local,
		successBodies: successBodies,
		planType:      planTypeMap[planType],
		executions:    executions,
		goroutines:    goroutines,
		delay:         time.Duration(delay) * time.Millisecond,
		overrideURL:   overrideURL,
	}

	relayUtil := newRelayUtil(config)

	relayUtil.sendRelays(config, request)
}

/* Relay Util Implementation */

const (
	productionStarterAppID    = "PRODUCTION_STARTER_APP_ID"
	productionStarterKey      = "PRODUCTION_STARTER_KEY"
	productionEnterpriseAppID = "PRODUCTION_ENTERPRISE_APP_ID"
	productionEnterpriseKey   = "PRODUCTION_ENTERPRISE_KEY"
	stagingStarterAppID       = "STAGING_STARTER_APP_ID"
	stagingStarterKey         = "STAGING_STARTER_KEY"
	stagingEnterpriseAppID    = "STAGING_ENTERPRISE_APP_ID"
	stagingEnterpriseKey      = "STAGING_ENTERPRISE_KEY"

	envProd    envType = "production"
	envStaging envType = "staging"
)

var planTypeMap = map[string]string{
	"starter":    "FREETIER_V0",
	"enterprise": "ENTERPRISE",
}

type (
	relayUtil struct {
		url           string
		secretKey     string
		request       string
		config        relayConfig
		httpClient    *http.Client
		results       chan relayResult
		executionTime time.Duration
	}

	options struct {
		appIDs  map[envType]map[string]portalAppData
		retries int
		timeout time.Duration
	}

	portalAppData struct {
		id  string
		key string
	}

	goroutinesConfig struct {
		goroutines int
		delay      time.Duration
	}

	relayConfig struct {
		appIDs        map[envType]map[string]portalAppData
		chain         string
		env           envType
		planType      string
		request       string
		overrideURL   string
		executions    int
		goroutines    int
		delay         time.Duration
		local         bool
		successBodies bool
	}

	envType string

	response struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      relay.ID    `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   relayError  `json:"error,omitempty"`
	}

	relayError struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	}

	relayResult struct {
		id          int32
		err         bool
		errReason   string
		successBody string
		latency     int32
	}
)

func newRelayUtil(config relayConfig) *relayUtil {
	r := relayUtil{
		request:    config.request,
		config:     config,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		results:    make(chan relayResult, config.executions),
	}

	url, secretKey := r.getURLStringAndKey()
	r.url = url
	r.secretKey = secretKey

	return &r
}

func gatherAppIDs() map[envType]map[string]portalAppData {
	return map[envType]map[string]portalAppData{
		envProd: {
			"FREETIER_V0": portalAppData{
				id:  env.MustGetString(productionStarterAppID),
				key: env.GetString(productionStarterKey, ""),
			},
			"ENTERPRISE": portalAppData{
				id:  env.MustGetString(productionEnterpriseAppID),
				key: env.GetString(productionEnterpriseKey, ""),
			},
		},
		envStaging: {
			"FREETIER_V0": portalAppData{
				id:  env.MustGetString(stagingStarterAppID),
				key: env.GetString(stagingStarterKey, ""),
			},
			"ENTERPRISE": portalAppData{
				id:  env.MustGetString(stagingEnterpriseAppID),
				key: env.GetString(stagingEnterpriseKey, ""),
			},
		},
	}
}

func getGoroutinesConfig(planType string, goroutines int, delay time.Duration) goroutinesConfig {
	if planType == "FREETIER_V0" {
		return goroutinesConfig{goroutines: 30,
			delay: 1_000 * time.Millisecond,
		}
	}

	return goroutinesConfig{
		goroutines: goroutines,
		delay:      delay,
	}
}

func runInGoroutines(config goroutinesConfig, executions int, jobFunc func()) {
	if err := config.validateConfig(); err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	sem := make(chan bool, config.goroutines)

	tasks := make(chan bool, executions)
	for i := 0; i < executions; i++ {
		tasks <- true
	}
	close(tasks)

	for i := 0; i < config.goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range tasks {
				sem <- true
				jobFunc()
				<-sem
			}
		}()

		// Delay between goroutine creation
		<-time.After(config.delay)
	}

	wg.Wait()
}

func (c *goroutinesConfig) validateConfig() error {
	if c.goroutines < 1 {
		return fmt.Errorf("goroutines must be greater than 0")
	}
	if c.delay < 0 {
		return fmt.Errorf("delay must be greater than or equal to 0")
	}
	return nil
}

func (r *relayUtil) sendRelays(relayConfig relayConfig, request string) {
	var counter atomic.Int32
	startTime := time.Now() // Capture the start time

	printConfig(relayConfig, r.url, request)

	// Create a new progress bar with the total count of relays
	bar := pb.StartNew(relayConfig.executions)
	blue := color.New(color.FgBlue).SprintFunc()

	// Customize the progress bar template to include the prefix with relay count
	bar.SetTemplateString(`{{string . "prefix"}} {{bar . "[" "=" ">" "_" "]"}} {{percent .}}`)
	bar.SetWidth(80)
	bar.SetMaxWidth(90)

	runInGoroutines(
		getGoroutinesConfig(relayConfig.planType, relayConfig.goroutines, relayConfig.delay),
		relayConfig.executions,
		func() {
			currentRelay := counter.Add(1)
			prefix := fmt.Sprintf("%s 📡 Sending relay %d of %d", blue("EXECUTION"), currentRelay, relayConfig.executions)
			bar.Set("prefix", prefix).Increment()

			result := relayResult{
				id: currentRelay,
			}

			startTime := time.Now() // Start time measurement

			response, err := r.makeJSONRPCReq() // Send the Relay Request

			latency := time.Since(startTime).Milliseconds() // Calculate latency

			if err != nil {
				result.err = true
				result.errReason = err.Error()
				r.results <- result
				return
			}
			if response == nil {
				result.err = true
				result.errReason = "response is nil"
				r.results <- result
				return
			}

			if response.Error.Message != "" {
				result.err = true
				result.errReason = fmt.Sprintf("code: %d, message: %s", response.Error.Code, response.Error.Message)
				r.results <- result
				return
			} else {
				responseJSON, err := json.Marshal(response.Result)
				if err != nil {
					result.err = true
					result.errReason = "failed to marshal response result to JSON"
					r.results <- result
					return
				}

				result.successBody = string(responseJSON)
				result.latency = int32(latency) // Store latency in the result
				r.results <- result
				return
			}
		},
	)

	r.executionTime = time.Since(startTime) // Capture the execution time
	bar.SetCurrent(int64(relayConfig.executions)).Set("prefix", "🎉 All relays sent!").Finish()
	close(r.results)

	r.logResults()
}

func printConfig(relayConfig relayConfig, urlString string, requestBody string) {
	// Parse the URL
	u, err := url.Parse(urlString)
	if err != nil {
		panic(err)
	}

	// Replace the password in the URL, if it exists
	if u.User != nil {
		username := u.User.Username()
		u.User = url.UserPassword(username, "***")
	}

	// Define color functions
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()
	magenta := color.New(color.FgMagenta).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	// Print the messages with colors and emojis
	fmt.Printf("%s 🚀 Sending %s relays to %s\n", green("INFO"), formatWithCommas(relayConfig.executions), maskAppID(u.String()))
	if relayConfig.overrideURL != "" {
		fmt.Printf("%s 🔀 Overriding URL with: %s\n", red("OVERRIDE"), maskAppID(relayConfig.overrideURL))
	}
	fmt.Printf("%s 🧵 Goroutines: %s\n", blue("DETAIL"), formatWithCommas(relayConfig.goroutines))
	fmt.Printf("%s ⏱️  Delay: %s\n", blue("DETAIL"), relayConfig.delay)
	fmt.Printf("%s 🌍 Env: %s\n", yellow("CONFIG"), relayConfig.env)
	fmt.Printf("%s 📝 Plan Type: %s\n", yellow("CONFIG"), relayConfig.planType)
	fmt.Printf("%s 🔗 Chain: %s\n", yellow("CONFIG"), relayConfig.chain)
	fmt.Printf("%s 📄 Request Body: %s\n\n", magenta("REQUEST"), requestBody)
}

func (r *relayUtil) logResults() {
	totalRelays := 0
	successfulRelays := 0
	failedRelays := 0
	successBodies := make(map[string]int)
	errorReasons := make(map[string]int)

	// Define color functions
	white := color.New(color.FgWhite).SprintfFunc()
	green := color.New(color.FgGreen).SprintfFunc()
	red := color.New(color.FgRed).SprintfFunc()
	yellow := color.New(color.FgYellow).SprintfFunc()
	blue := color.New(color.FgBlue).SprintfFunc()

	var formattedExecutionTime string
	if r.executionTime.Seconds() >= 1 {
		formattedExecutionTime = fmt.Sprintf("%.2fs", r.executionTime.Seconds())
	} else {
		formattedExecutionTime = fmt.Sprintf("%dms", r.executionTime.Milliseconds())
	}

	// Collect latencies for successful relays
	var latencies []int32

	for result := range r.results {
		totalRelays++
		if result.err {
			failedRelays++
			errorReasons[result.errReason]++
		} else {
			successfulRelays++
			successBodies[result.successBody]++
			if result.latency != 0 {
				latencies = append(latencies, result.latency)
			}
		}

	}

	successRate := float64(successfulRelays) / float64(totalRelays) * 100
	failureRate := float64(failedRelays) / float64(totalRelays) * 100

	// Determine color based on failure rate
	var failureColorFunc func(format string, a ...interface{}) string
	switch {
	case failureRate > 5:
		failureColorFunc = red
	case failureRate > 1:
		failureColorFunc = yellow
	default:
		failureColorFunc = white
	}

	// Determine color based on success rate
	var successColorFunc func(format string, a ...interface{}) string
	switch {
	case successRate >= 99:
		successColorFunc = green
	case successRate >= 95:
		successColorFunc = yellow
	default:
		successColorFunc = red
	}

	// Function to select color based on latency
	colorForLatency := func(latency int32) func(format string, a ...interface{}) string {
		switch {
		case latency > 800:
			return red
		case latency > 250:
			return yellow
		case latency > 90:
			return green
		default:
			return blue
		}
	}

	// Calculate average, highest, lowest, and p90 latency
	var totalLatency int64
	highestLatency := int32(math.MinInt32)
	lowestLatency := int32(math.MaxInt32)
	for _, latency := range latencies {
		totalLatency += int64(latency)
		if latency > highestLatency {
			highestLatency = latency
		}
		if latency < lowestLatency {
			lowestLatency = latency
		}
	}
	averageLatency := float64(totalLatency) / float64(len(latencies))

	// Sort latencies to find p90
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	var p90Latency int32
	if len(latencies) > 0 {
		p90Index := int(float64(len(latencies)) * 0.9)
		if p90Index == 0 {
			p90Latency = latencies[0] // If there's only one latency, it's also the p90
		} else {
			p90Latency = latencies[p90Index-1]
		}
	}

	fmt.Printf("\n")
	fmt.Println(blue("📊 RESULTS"))
	fmt.Printf("⏳ Total time taken: %s\n", formattedExecutionTime)
	fmt.Println("🔢 Total relays:", formatWithCommas(totalRelays))
	fmt.Printf("✅ Successful relays: %s\n", successColorFunc("%s", formatWithCommas(successfulRelays)))
	fmt.Printf("❌ Failed relays: %s\n", failureColorFunc("%s", formatWithCommas(failedRelays)))
	fmt.Printf("📈 Success rate: %s\n", successColorFunc("%.2f%%", successRate))
	fmt.Printf("📉 Failure rate: %s\n", failureColorFunc("%.2f%%", failureRate))

	if r.config.successBodies {
		fmt.Printf("\n")
		if len(successBodies) > 0 {
			fmt.Println(green("Successful response bodies and their occurrences:"))
			for successBody, count := range successBodies {
				fmt.Printf("✅ %d occurrence%s - %s\n", count, suffixBasedOnLength(count), successBody)
			}
		}
	}

	if len(errorReasons) > 0 {
		fmt.Printf("\n")
		fmt.Println(red("Error reasons:"))
		for errReason, count := range errorReasons {
			fmt.Printf("🚫 %d occurence%s - %s\n", count, suffixBasedOnLength(count), errReason)
		}
	}

	// Log latencies
	fmt.Printf("\n")
	fmt.Println(blue("🕒 LATENCIES"))
	fmt.Printf("🔊 P90 latency: %s\n", colorForLatency(int32(p90Latency))("%dms", p90Latency))
	fmt.Printf("🐕 Average latency: %s\n", colorForLatency(int32(averageLatency))("%.2fms", averageLatency))
	fmt.Printf("🦅 Lowest latency: %s\n", colorForLatency(int32(lowestLatency))("%dms", lowestLatency))
	fmt.Printf("🐢 Highest latency: %s\n", colorForLatency(int32(highestLatency))("%dms", highestLatency))
}

func (r *relayUtil) getURLStringAndKey() (url string, key string) {
	if r.config.overrideURL != "" {
		return r.config.overrideURL, ""
	}

	appID := r.config.appIDs[r.config.env][r.config.planType].id

	if strings.Contains(string(appID), "dummy") {
		fmt.Printf("⚠️ The relevant app ID for %s and %s is not set.\n", r.config.env, r.config.planType)
		if newAppID, newKey := askForEnvVarUpdate(string(r.config.env), string(r.config.planType)); newAppID != "" {
			newAppIDs := r.config.appIDs
			newAppIDs[r.config.env][r.config.planType] = portalAppData{
				id:  newAppID,
				key: newKey,
			}
			r.config.appIDs = newAppIDs

			return r.getURLStringAndKey() // Recursively call to get the updated URL and key
		}
		fmt.Println("Exiting program. Please set the correct Portal App ID to proceed.")
		os.Exit(1)
	}

	if r.config.local {
		url = fmt.Sprintf("http://%s.localhost:3000/v1/%s", r.config.chain, appID)
	} else {
		var domain string
		if r.config.env == envProd {
			domain = "city"
		} else {
			domain = "town"
		}
		url = fmt.Sprintf("https://%s.rpc.grove.%s/v1/%s", r.config.chain, domain, appID)
	}

	key = r.config.appIDs[r.config.env][r.config.planType].key

	return url, key
}

func suffixBasedOnLength(count int) string {
	if count > 1 {
		return "s"
	}
	return ""
}

func maskAppID(urlString string) string {
	// Parse the URL
	u, err := url.Parse(urlString)
	if err != nil {
		return urlString // If there's an error parsing, return the original string
	}

	maskedURL := u.Scheme + "://"

	// Mask the password if it exists
	if u.User != nil {
		username := u.User.Username()
		_, hasPassword := u.User.Password()
		if hasPassword {
			// Build userInfo with masked password
			maskedURL += username + ":******@"
		} else {
			// Include username only if there's no password
			maskedURL += username + "@"
		}
	}

	// Add host
	maskedURL += u.Host

	// Mask the AppID if it's present in the path
	parts := strings.Split(u.Path, "/")
	if len(parts) > 0 {
		lastPartIndex := len(parts) - 1
		if len(parts[lastPartIndex]) == 8 {
			parts[lastPartIndex] = "******"
		}
		u.Path = strings.Join(parts, "/")
	}

	// Add path
	maskedURL += u.Path

	return maskedURL
}

func formatWithCommas(number int) string {
	in := strconv.Itoa(number)
	out := make([]rune, 0, len(in)+(len(in)-1)/3)

	for i, c := range in {
		if i > 0 && (len(in)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}

	return string(out)
}

func (r *relayUtil) makeJSONRPCReq() (*response, error) {
	req, err := http.NewRequest(http.MethodPost, r.url, bytes.NewBuffer([]byte(r.request)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if r.config.overrideURL == "" && r.secretKey != "" {
		req.Header.Set("Authorization", r.secretKey)
	}

	httpResp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	var resp response
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

var (
	requiredEnvVars = map[string]struct{}{
		productionStarterAppID:    {},
		productionEnterpriseAppID: {},
		stagingStarterAppID:       {},
		stagingEnterpriseAppID:    {},
	}
	optionalEnvVars = map[string]struct{}{
		productionStarterKey:    {},
		productionEnterpriseKey: {},
		stagingStarterKey:       {},
		stagingEnterpriseKey:    {},
	}
)

func Start() {
	checkEnvFile()
}

func checkEnvFile() {
	_, err := os.Stat(".env")
	if os.IsNotExist(err) {
		promptUser()
	}
}

func promptUser() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("👋 Welcome to the Relay Util app! It looks like you're running the app for the first time.\n❓ We need to gather a few variables to get started.\n🌿 In order to send relays for a specific environment and plan type combination you will need to enter a Portal App ID for that combination.\n➡️ You may skip entering a Portal App ID but you will not be able to send relays for the skipped environment and plan type combination until you enter a valid Portal App ID.\n🚀 Would you like to proceed?\n(yes/no): ")

	text, _ := reader.ReadString('\n')
	text = strings.ReplaceAll(text, "\n", "")
	if strings.ToLower(text) == "yes" {
		createEnvFile()
	}
}

func createEnvFile() {
	clearConsole()

	file, err := os.Create(".env")
	if err != nil {
		fmt.Println("🚫 Error creating .env file:", err)
		return
	}
	defer file.Close()

	// Set up a channel to listen for interrupt signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Set up a defer function to handle cleanup on interrupt
	defer func() {
		signal.Stop(c)

		if r := recover(); r != nil {
			fmt.Println("🚫 The Relay Util setup was aborted before being completed. Removing the .env file.")
			removeErr := os.Remove(".env")
			if removeErr != nil {
				fmt.Println("🚫 Failed to remove the .env file:", removeErr)
			}
			panic(r) // re-throw the panic after cleaning up
		}
	}()

	go func() {
		<-c // Block until a signal is received.
		fmt.Println("🚫 Interrupt signal received. Removing the .env file.")
		if removeErr := os.Remove(".env"); removeErr != nil {
			fmt.Println("🚫 Failed to remove the .env file:", removeErr)
		}
		os.Exit(1)
	}()

	envVarPrompts := []struct {
		key, description, dummy string
		required                bool
	}{
		{productionStarterAppID, "🌿 Enter your PRODUCTION STARTER Portal App ID (required to send relays using this environment and plan combination, press Enter to skip with a dummy value):\n", "dummy_starter_app_id", true},
		{productionStarterKey, "🔑 Enter your PRODUCTION STARTER Secret Key (must be set if Portal App requires a secret key to send relays, press Enter to skip):\n", "", false},
		{productionEnterpriseAppID, "🌿 Enter your PRODUCTION ENTERPRISE Portal App ID (required to send relays using this environment and plan combination, press Enter to skip with a dummy value):\n", "dummy_enterprise_app_id", true},
		{productionEnterpriseKey, "🔑 Enter your PRODUCTION ENTERPRISE Secret Key (must be set if Portal App requires a secret key to send relays, press Enter to skip):\n", "", false},
		{stagingStarterAppID, "🌿 Enter your STAGING STARTER Portal App ID (required to send relays using this environment and plan combination, press Enter to skip with a dummy value):\n", "dummy_staging_starter_app_id", true},
		{stagingStarterKey, "🔑 Enter your STAGING STARTER Secret Key (must be set if Portal App requires a secret key to send relays, press Enter to skip):\n", "", false},
		{stagingEnterpriseAppID, "🌿 Enter your STAGING ENTERPRISE Portal App ID (required to send relays using this environment and plan combination, press Enter to skip with a dummy value):\n", "dummy_staging_enterprise_app_id", true},
		{stagingEnterpriseKey, "🔑 Enter your STAGING ENTERPRISE Secret Key (must be set if Portal App requires a secret key to send relays, press Enter to skip):\n", "", false},
	}

	reader := bufio.NewReader(os.Stdin)
	var skipNextKeyPrompt bool

	for i, prompt := range envVarPrompts {
		if skipNextKeyPrompt && !prompt.required {
			skipNextKeyPrompt = false
			continue
		}

		fmt.Print(prompt.description)
		value, _ := reader.ReadString('\n')
		value = strings.TrimSpace(value)

		if value == "" {
			if prompt.required {
				value = prompt.dummy
				clearConsole()
				if i < len(envVarPrompts)-1 && !envVarPrompts[i+1].required {
					// If the next prompt is for a secret key, skip it
					skipNextKeyPrompt = true
				}
			} else {
				// If it's an optional key and the value is empty, just continue
				continue
			}
		}

		_, err := file.WriteString(fmt.Sprintf("%s=%s\n", prompt.key, value))
		if err != nil {
			fmt.Println("🚫 Error writing to .env file:", err)
			return
		}

		os.Setenv(prompt.key, value)
		if !prompt.required {
			clearConsole()
		}
	}

	clearConsole()
	fmt.Println("📡 .env file has been created and populated; you are ready to send relays!")
	fmt.Println("❔ To see the documentation for this app, run `relay-util --help` or `relay-util -h`")

	// Gracefully exit the program
	os.Exit(0)
}

func clearConsole() {
	fmt.Print("\033[H\033[2J")
}

func askForEnvVarUpdate(env, planType string) (string, string) {
	reader := bufio.NewReader(os.Stdin)
	var appIDValue, appKeyValue string

	for {
		fmt.Printf("❓ Would you like to set the Portal App ID for %s and %s now? (yes/no):\n", env, planType)
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(strings.ToLower(text))

		if text == "yes" {
			appIDValue, appKeyValue = updateEnvFile(env, planType)
			break
		} else if text == "no" {
			fmt.Println("🚫 Exiting program. Please set the correct Portal App ID to proceed.")
			os.Exit(1)
		} else {
			fmt.Println("🚫 Invalid input. Please type 'yes' or 'no'.")
		}
	}

	return appIDValue, appKeyValue
}

func updateEnvFile(env, planType string) (string, string) {
	envFilePath := ".env"
	readFile, err := os.ReadFile(envFilePath)
	if err != nil {
		fmt.Println("🚫 Error reading .env file:", err)
		return "", ""
	}

	contents := string(readFile)

	reader := bufio.NewReader(os.Stdin)
	appIDKey, appKeyKey := getAppIDAndKeyKeys(env, planType)

	fmt.Printf("🌿 Enter the Portal App ID for %s and %s:\n", env, planType)
	appIDValue, _ := reader.ReadString('\n')
	appIDValue = strings.TrimSpace(appIDValue)

	if appIDValue != "" {
		contents = updateEnvValue(contents, appIDKey, appIDValue)
		os.Setenv(appIDKey, appIDValue)
	}

	fmt.Printf("🔑 Enter the Secret Key for %s and %s (optional, press Enter to skip):\n", env, planType)
	appKeyValue, _ := reader.ReadString('\n')
	appKeyValue = strings.TrimSpace(appKeyValue)

	if appKeyValue != "" {
		contents = updateEnvValue(contents, appKeyKey, appKeyValue)
		os.Setenv(appKeyKey, appKeyValue)
	}

	err = os.WriteFile(envFilePath, []byte(contents), 0644)
	if err != nil {
		fmt.Println("🚫 Error writing to .env file:", err)
		return "", ""
	}

	clearConsole()
	fmt.Println("📡 .env file has been updated!")
	return appIDValue, appKeyValue
}

func updateEnvValue(contents, key, newValue string) string {
	lines := strings.Split(contents, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			lines[i] = key + "=" + newValue
			return strings.Join(lines, "\n")
		}
	}
	// If the key does not exist, append it to the contents
	return contents + key + "=" + newValue + "\n"
}

func getAppIDAndKeyKeys(env, planType string) (appIDKey, appKeyKey string) {
	switch env {
	case "production":
		if planType == "starter" {
			appIDKey = productionStarterAppID
			appKeyKey = productionStarterKey
		} else {
			appIDKey = productionEnterpriseAppID
			appKeyKey = productionEnterpriseKey
		}
	case "staging":
		if planType == "starter" {
			appIDKey = stagingStarterAppID
			appKeyKey = stagingStarterKey
		} else {
			appIDKey = stagingEnterpriseAppID
			appKeyKey = stagingEnterpriseKey
		}
	}
	return appIDKey, appKeyKey
}
