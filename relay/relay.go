package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cheggaaa/pb/v3"
	"github.com/commoddity/relay-util/env"
	"github.com/commoddity/relay-util/setup"
	"github.com/fatih/color"
)

type (
	ID struct {
		string   string
		number   int
		isNumber bool
	}

	Response struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      ID          `json:"id"`
		Result  interface{} `json:"result,omitempty"`
		Error   RelayError  `json:"error,omitempty"`
	}

	RelayError struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	}

	RelayResult struct {
		ID          int32
		Err         bool
		ErrReason   string
		SuccessBody string
		Latency     int32
	}

	Config struct {
		Env           env.EnvType
		PlanType      env.PlanType
		Chain         string
		Request       string
		OverrideURL   string
		Executions    int
		Goroutines    int
		Delay         time.Duration
		Local         bool
		SuccessBodies bool
	}

	Util struct {
		HTTPClient    *http.Client
		AppIDs        map[env.EnvType]map[env.PlanType]env.PortalAppData
		Env           env.EnvType
		PlanType      env.PlanType
		Chain         string
		URL           string
		SecretKey     string
		Request       string
		OverrideURL   string
		Executions    int
		Goroutines    int
		Delay         time.Duration
		ExecTime      time.Duration
		Local         bool
		SuccessBodies bool
		ResultChan    chan RelayResult
	}

	goroutinesConfig struct {
		goroutines int
		delay      time.Duration
	}
)

// NewRelayUtil creates a new instance of the Relay Util.
func NewRelayUtil(config Config) *Util {
	util := &Util{
		HTTPClient:    &http.Client{Timeout: 5 * time.Second},
		AppIDs:        env.GatherAppIDs(),
		ResultChan:    make(chan RelayResult, config.Executions),
		Env:           config.Env,
		PlanType:      config.PlanType,
		Chain:         config.Chain,
		Request:       config.Request,
		OverrideURL:   config.OverrideURL,
		Executions:    config.Executions,
		Goroutines:    config.Goroutines,
		Delay:         config.Delay,
		Local:         config.Local,
		SuccessBodies: config.SuccessBodies,
	}

	util.setURLStringAndKey()

	return util
}

// SendRelays sends the relays to the Portal API and stores the results in the ResultChan.
func (u *Util) SendRelays() {
	var counter atomic.Int32
	startTime := time.Now() // Capture the start time

	// Create a new progress bar with the total count of relays
	bar := pb.StartNew(u.Executions)
	blue := color.New(color.FgBlue).SprintFunc()

	// Customize the progress bar template to include the prefix with relay count
	bar.SetTemplateString(`{{string . "prefix"}} {{bar . "[" "=" ">" "_" "]"}} {{percent .}}`)
	bar.SetWidth(80)
	bar.SetMaxWidth(90)

	runInGoroutines(
		getGoroutinesConfig(u.PlanType, u.Goroutines, u.Delay),
		u.Executions,
		func() {
			currentRelay := counter.Add(1)
			prefix := fmt.Sprintf("%s 📡 Sending relay %d of %d", blue("EXECUTION"), currentRelay, u.Executions)
			bar.Set("prefix", prefix).Increment()

			result := RelayResult{
				ID: currentRelay,
			}

			startTime := time.Now() // Start time measurement

			response, err := u.makeJSONRPCReq()             // Make the JSON-RPC request
			latency := time.Since(startTime).Milliseconds() // Calculate latency

			if err != nil {
				result.Err = true
				result.ErrReason = err.Error()
				u.ResultChan <- result
				return
			}
			if response == nil {
				result.Err = true
				result.ErrReason = "response is nil"
				u.ResultChan <- result
				return
			}

			if response.Error.Message != "" {
				result.Err = true
				result.ErrReason = fmt.Sprintf("code: %d, message: %s", response.Error.Code, response.Error.Message)
				u.ResultChan <- result
				return
			} else {
				responseJSON, err := json.Marshal(response.Result)
				if err != nil {
					result.Err = true
					result.ErrReason = "failed to marshal response result to JSON"
					u.ResultChan <- result
					return
				}

				result.SuccessBody = string(responseJSON)
				result.Latency = int32(latency) // Store latency in the result
				u.ResultChan <- result
				return
			}
		},
	)

	u.ExecTime = time.Since(startTime) // Capture the execution time

	bar.SetCurrent(int64(u.Executions)).Set("prefix", "🎉 All relays sent!").Finish()

	close(u.ResultChan)
}

// setURLStringAndKey sets the URL and SecretKey fields on the Util struct.
func (u *Util) setURLStringAndKey() {
	if u.OverrideURL != "" {
		u.URL = u.OverrideURL
		return
	}

	appID := u.AppIDs[u.Env][u.PlanType].ID

	if updateAppIDAndKeyIfDummy(u, appID) {
		u.setURLStringAndKey() // Recursively call to get the updated URL and key
	}

	if u.Local {
		u.URL = fmt.Sprintf("http://%s.localhost:3000/v1/%s", u.Chain, appID)
	} else {
		var domain string
		if u.Env == env.EnvProd {
			domain = "city"
		} else {
			domain = "town"
		}
		u.URL = fmt.Sprintf("https://%s.rpc.grove.%s/v1/%s", u.Chain, domain, appID)
	}

	u.SecretKey = u.AppIDs[u.Env][u.PlanType].Key
}

// IDFromString creates an ID from a string.
func IDFromString(id string) ID {
	return ID{string: id, isNumber: false}
}

// IDFromInt creates an ID from an int.
func IDFromInt(id int) ID {
	return ID{number: id, isNumber: true}
}

// UnmarshalJSON unmarshals an ID from JSON.
func (i *ID) UnmarshalJSON(data []byte) error {
	var intID int
	if err := json.Unmarshal(data, &intID); err == nil {
		i.number = intID
		i.isNumber = true
		return nil
	}

	var stringID string
	if err := json.Unmarshal(data, &stringID); err == nil {
		i.string = stringID
		return nil
	}

	return fmt.Errorf("error unmarshalling ID: %s", string(data))
}

func (i ID) MarshalJSON() ([]byte, error) {
	if i.isNumber {
		return json.Marshal(i.number)
	}
	return json.Marshal(i.string)
}

func (i ID) String() string {
	if i.isNumber {
		return fmt.Sprintf("%v", i.number)
	}
	return i.string
}

// makeJSONRPCReq makes a JSON-RPC request to the Portal API.
func (u *Util) makeJSONRPCReq() (*Response, error) {
	req, err := http.NewRequest(http.MethodPost, u.URL, bytes.NewBuffer([]byte(u.Request)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if u.OverrideURL == "" && u.SecretKey != "" {
		req.Header.Set("Authorization", u.SecretKey)
	}

	httpResp, err := u.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer httpResp.Body.Close()
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	var resp Response
	err = json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// getGoroutinesConfig returns the goroutines config based on the plan type.
func getGoroutinesConfig(planType env.PlanType, goroutines int, delay time.Duration) goroutinesConfig {
	if planType == env.PlanTypeStarter {
		return goroutinesConfig{
			goroutines: 30,
			delay:      1_000 * time.Millisecond,
		}
	}

	return goroutinesConfig{
		goroutines: goroutines,
		delay:      delay,
	}
}

// runInGoroutines runs a function in goroutines.
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

// validateConfig validates the goroutines config.
func (u *goroutinesConfig) validateConfig() error {
	if u.goroutines < 1 {
		return fmt.Errorf("goroutines must be greater than 0")
	}
	if u.delay < 0 {
		return fmt.Errorf("delay must be greater than or equal to 0")
	}
	return nil
}

// updateAppIDAndKeyIfDummy updates the AppID and Key if the AppID is a dummy value.
func updateAppIDAndKeyIfDummy(u *Util, appID string) (updated bool) {
	if strings.Contains(appID, "dummy") {
		if newAppID, newKey := setup.PromptUpdateDummyAppIDs(string(u.Env), string(u.PlanType)); newAppID != "" {
			newAppIDs := u.AppIDs

			newData := env.PortalAppData{ID: newAppID, Key: newKey}
			newAppIDs[u.Env][u.PlanType] = newData

			u.AppIDs = newAppIDs

			return true
		}
	}

	return false
}
