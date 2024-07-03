package log

import (
	"encoding/hex"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/commoddity/relay-util/relay"
	"github.com/fatih/color"
)

// PrintConfig prints the relay configuration to the console.
func PrintConfig(u *relay.Util) {
	// Parse the URL
	urlStr, err := url.Parse(u.URL)
	if err != nil {
		panic(err)
	}

	// Replace the password in the URL, if it exists
	if urlStr.User != nil {
		username := urlStr.User.Username()
		urlStr.User = url.UserPassword(username, "***")
	}

	// Define color functions
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	blue := color.New(color.FgBlue).SprintFunc()
	magenta := color.New(color.FgMagenta).SprintFunc()
	red := color.New(color.FgRed).SprintFunc()

	// Print the messages with colors and emojis
	fmt.Printf("%s 🚀 Sending %s relays to %s\n", green("INFO"), formatWithCommas(u.Executions), maskAppID(urlStr.String()))
	if u.OverrideURL != "" {
		fmt.Printf("%s 🔀 Overriding URL with: %s\n", red("OVERRIDE"), maskAppID(u.OverrideURL))
	}
	fmt.Printf("%s 🧵 Goroutines: %s\n", blue("DETAIL"), formatWithCommas(u.Goroutines))
	fmt.Printf("%s ⏱️  Delay: %s\n", blue("DETAIL"), u.Delay)
	fmt.Printf("%s ⏳ Timeout: %s\n", blue("DETAIL"), u.Timeout)
	fmt.Printf("%s 🌍 Env: %s\n", yellow("CONFIG"), u.Env)
	fmt.Printf("%s 📝 Plan Type: %s\n", yellow("CONFIG"), u.PlanType)
	fmt.Printf("%s 🔗 Chain: %s\n", yellow("CONFIG"), u.Chain)
	fmt.Printf("%s 📄 Request Body: %s\n\n", magenta("REQUEST"), u.Request)
}

// LogResults logs the results of the relay execution to the console
// from the ResultChan, which is populated by the SendRelays function.
func LogResults(u *relay.Util) {
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
	if u.ExecTime.Seconds() >= 1 {
		formattedExecutionTime = fmt.Sprintf("%.2fs", u.ExecTime.Seconds())
	} else {
		formattedExecutionTime = fmt.Sprintf("%dms", u.ExecTime.Milliseconds())
	}

	// Collect latencies for successful relays
	var latencies []int32

	for result := range u.ResultChan {
		totalRelays++
		if result.Err {
			failedRelays++
			errorReasons[result.ErrReason]++
		} else {
			successfulRelays++
			successBodies[result.SuccessBody]++
			if result.Latency != 0 {
				latencies = append(latencies, result.Latency)
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

	if u.SuccessBodies {
		fmt.Printf("\n")
		if len(successBodies) > 0 {
			fmt.Println(green("Successful response bodies and their occurrences:"))

			// Convert map to slice for sorting
			type kv struct {
				Key   string
				Value int
			}

			var ss []kv
			for k, v := range successBodies {
				ss = append(ss, kv{k, v})
			}

			// Sort by converting hex to number where possible, else by string
			sort.Slice(ss, func(i, j int) bool {
				decodedI, okI := hexToTextOrNumber(ss[i].Key)
				decodedJ, okJ := hexToTextOrNumber(ss[j].Key)
				if okI && okJ {
					numI, _ := strconv.Atoi(strings.ReplaceAll(decodedI, ",", ""))
					numJ, _ := strconv.Atoi(strings.ReplaceAll(decodedJ, ",", ""))
					return numI < numJ
				}
				return ss[i].Key < ss[j].Key
			})

			for _, kv := range ss {
				str := fmt.Sprintf("✅ %d occurrence%s - %s", kv.Value, suffixBasedOnLength(kv.Value), kv.Key)

				if decodedHex, ok := hexToTextOrNumber(kv.Key); ok {
					str = fmt.Sprintf("%s (%s)", str, decodedHex)
				}

				fmt.Println(str)
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

// formatWithCommas formats a number with commas
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

// maskAppID masks the App ID in the URL
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

// suffixBasedOnLength returns a suffix based on the length of the count
func suffixBasedOnLength(count int) string {
	if count > 1 {
		return "s"
	}
	return ""
}

// hexToTextOrNumber tries to convert a hex string to its text or number representation.
// If the input is not a valid hex string, it returns the original input.
func hexToTextOrNumber(hexStr string) (string, bool) {
	// Remove the "0x" prefix if present and any potential quotes
	trimmedHexStr := strings.TrimPrefix(strings.Trim(hexStr, "\""), "0x")

	// Check if the string is a hex number and convert
	if num, err := strconv.ParseInt(trimmedHexStr, 16, 64); err == nil {
		formattedNum := formatWithCommas(int(num))
		return formattedNum, true
	}

	// Attempt to decode hex to bytes assuming it could be a hex-encoded text
	bytes, err := hex.DecodeString(trimmedHexStr)
	if err != nil {
		// Not a valid hex string, return original
		return hexStr, false
	}

	// Convert bytes to string and return
	return string(bytes), true
}
