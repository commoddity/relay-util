package setup

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/commoddity/relay-util/env"
)

// Start checks if the .env file exists, if not, prompts the user to create it
func Start() {
	checkEnvFile()
}

// checkEnvFile checks if the .env file exists, if not, prompts the user to create it
func checkEnvFile() {
	_, err := os.Stat(env.EnvPath)
	if os.IsNotExist(err) {
		promptUser()
	}
}

// promptUser prompts the user to create the .env file
func promptUser() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("👋 Welcome to the Relay Util app! It looks like you're running the app for the first time.\n❓ We need to gather a few variables to get started.\n🌿 In order to send relays for a specific environment and plan type combination you will need to enter a Portal App ID for that combination.\n👀 You may skip entering a Portal App ID but you will not be able to send relays for the skipped environment and plan type combination until you enter a valid Portal App ID.\n🚀 Would you like to proceed?\n(yes/no): ")

	// Set up a defer function to handle cleanup on interrupt
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("🚫 The Relay Util setup was aborted before being completed. Removing the .env file.")
			removeErr := os.Remove(env.EnvPath)
			if removeErr != nil {
				fmt.Println("🚫 Failed to remove the .env file:", removeErr)
			}
			os.Exit(1)
		}
	}()

	text, _ := reader.ReadString('\n')
	text = strings.ReplaceAll(text, "\n", "")
	if strings.ToLower(text) == "yes" {
		createEnvFile()
	}

	if text == "no" {
		panic("🚫 Exiting program. Please set the correct Portal App IDs to proceed.")
	}
}

func createEnvFile() {
	clearConsole()

	file, err := os.OpenFile(env.EnvPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		fmt.Println("🚫 Error creating .env file:", err)
		return
	}

	// Set up a channel to listen for interrupt signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	go func() {
		<-c // Block until a signal is received.
		fmt.Println("🚫 Interrupt signal received. Removing the .env file.")
		if removeErr := os.Remove(env.EnvPath); removeErr != nil {
			fmt.Println("🚫 Failed to remove the .env file:", removeErr)
		}
		file.Close()
		os.Exit(1)
	}()

	envVarPrompts := []struct {
		key, description, dummy string
		required                bool
	}{
		{env.ProductionStarterAppID, "🌿 Enter your PRODUCTION STARTER Portal App ID (required to send relays using this environment and plan combination, press Enter to skip with a dummy value):\n", "dummy_starter_app_id", true},
		{env.ProductionStarterKey, "🔑 Enter your PRODUCTION STARTER Secret Key (must be set if Portal App requires a secret key to send relays, press Enter to skip):\n", "", false},
		{env.ProductionEnterpriseAppID, "🌿 Enter your PRODUCTION ENTERPRISE Portal App ID (required to send relays using this environment and plan combination, press Enter to skip with a dummy value):\n", "dummy_enterprise_app_id", true},
		{env.ProductionEnterpriseKey, "🔑 Enter your PRODUCTION ENTERPRISE Secret Key (must be set if Portal App requires a secret key to send relays, press Enter to skip):\n", "", false},
		{env.StagingStarterAppID, "🌿 Enter your STAGING STARTER Portal App ID (required to send relays using this environment and plan combination, press Enter to skip with a dummy value):\n", "dummy_staging_starter_app_id", true},
		{env.StagingStarterKey, "🔑 Enter your STAGING STARTER Secret Key (must be set if Portal App requires a secret key to send relays, press Enter to skip):\n", "", false},
		{env.StagingEnterpriseAppID, "🌿 Enter your STAGING ENTERPRISE Portal App ID (required to send relays using this environment and plan combination, press Enter to skip with a dummy value):\n", "dummy_staging_enterprise_app_id", true},
		{env.StagingEnterpriseKey, "🔑 Enter your STAGING ENTERPRISE Secret Key (must be set if Portal App requires a secret key to send relays, press Enter to skip):\n", "", false},
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

	file.Close()
	clearConsole()
	fmt.Println("📡 .env file has been created and populated; you are ready to send relays!")
	fmt.Println("❔ To see the documentation for this app, run `relay-util --help` or `relay-util -h`")

	// Gracefully exit the program
	os.Exit(0)
}

// PromptUpdateDummyAppIDs prompts the user to update the dummy App ID
func PromptUpdateDummyAppIDs(env, planType string) (string, string) {
	fmt.Printf("⚠️ The relevant app ID for %s and %s is not set.\n", env, planType)

	if newAppID, newKey := askForEnvVarUpdate(string(env), string(planType)); newAppID != "" {
		return newAppID, newKey
	}

	fmt.Println("Exiting program. Please set the correct Portal App ID to proceed.")
	os.Exit(1)
	return "", ""
}

// askForEnvVarUpdate asks the user if they want to update the environment variable
func askForEnvVarUpdate(env, planType string) (string, string) {
	reader := bufio.NewReader(os.Stdin)
	var appIDValue, appKeyValue string

	for {
		fmt.Printf("❓ The relevant app ID for %s and %s is not set.\nWould you like to set the Portal App ID for %s and %s now? (yes/no):\n", env, planType, env, planType)
		text, _ := reader.ReadString('\n')
		text = strings.TrimSpace(strings.ToLower(text))

		switch text {
		case "yes":
			clearConsole()
			appIDValue, appKeyValue = updateEnvFile(env, planType)
			return appIDValue, appKeyValue
		case "no":
			fmt.Println("🚫 Exiting program. Please set the correct Portal App ID to proceed.")
			os.Exit(1)
		default:
			fmt.Println("🚫 Invalid input. Please type 'yes' or 'no'.")
			askForEnvVarUpdate(env, planType)
		}
	}
}

// updateEnvFile updates the .env file with the new App ID and Key
func updateEnvFile(envStr, planType string) (string, string) {
	envFilePath := env.EnvPath
	readFile, err := os.ReadFile(envFilePath)
	if err != nil {
		fmt.Println("🚫 Error reading .env file:", err)
		return "", ""
	}

	contents := string(readFile)

	reader := bufio.NewReader(os.Stdin)
	appIDKey, appKeyKey := getAppIDAndKeyKeys(env.EnvType(envStr), env.PlanType(planType))

	fmt.Printf("🌿 Enter the Portal App ID for %s and %s:\n", envStr, planType)
	appIDValue, _ := reader.ReadString('\n')
	appIDValue = strings.TrimSpace(appIDValue)

	if appIDValue != "" {
		contents = updateEnvValue(contents, appIDKey, appIDValue)
		os.Setenv(appIDKey, appIDValue)
	}

	fmt.Printf("🔑 Enter the Secret Key for %s and %s (optional, press Enter to skip):\n", envStr, planType)
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

// updateEnvValue updates the value of an environment variable in the .env file
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

// clearConsole clears the console
func clearConsole() {
	fmt.Print("\033[H\033[2J")
}

// getAppIDAndKeyKeys gets the App ID and Key keys for the environment and plan type
func getAppIDAndKeyKeys(envStr env.EnvType, planType env.PlanType) (appIDKey, appKeyKey string) {
	switch envStr {
	case env.EnvProd:
		if planType == env.PlanTypeStarter {
			appIDKey = env.ProductionStarterAppID
			appKeyKey = env.ProductionStarterKey
		} else {
			appIDKey = env.ProductionEnterpriseAppID
			appKeyKey = env.ProductionEnterpriseKey
		}
	case env.EnvStaging:
		if planType == env.PlanTypeStarter {
			appIDKey = env.StagingStarterAppID
			appKeyKey = env.StagingStarterKey
		} else {
			appIDKey = env.StagingEnterpriseAppID
			appKeyKey = env.StagingEnterpriseKey
		}
	}

	return appIDKey, appKeyKey
}
