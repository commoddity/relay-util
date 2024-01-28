// Package environment is a set of functions to get env values or their default
// It has the autoload from .env files
package env

import (
	"fmt"
	"os"
	"strings"

	// autoload env vars
	_ "github.com/joho/godotenv/autoload"
)

// MustGetString gets the required environment var as a string and panics if it is not present
func MustGetString(varName string) string {
	val, _ := os.LookupEnv(varName)
	if val == "" {
		panic(fmt.Sprintf("environment error (string): required env var %s not found", varName))
	}

	return val
}

// GetString gets the environment var as a string
func GetString(varName string, defaultValue string) string {
	val, _ := os.LookupEnv(varName)
	if val == "" {
		return defaultValue
	}

	return val
}

// MustGetStringSlice gets the required environment var as a string slice and panics if it is not present
func MustGetStringSlice(varName string) []string {
	rawString := MustGetString(varName)
	stringSlice := strings.Split(rawString, ",")

	return stringSlice
}

// GetStringSlice gets the environment var as a string slice
func GetStringSlice(varName, defaultValue string) []string {
	rawString := GetString(varName, defaultValue)
	stringSlice := strings.Split(rawString, ",")

	return stringSlice
}
