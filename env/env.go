// Package environment is a set of functions to get env values or their default
// It has the autoload from .env files
package env

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	ProductionStarterAppID    = "PRODUCTION_STARTER_APP_ID"
	ProductionStarterKey      = "PRODUCTION_STARTER_KEY"
	ProductionEnterpriseAppID = "PRODUCTION_ENTERPRISE_APP_ID"
	ProductionEnterpriseKey   = "PRODUCTION_ENTERPRISE_KEY"
	StagingStarterAppID       = "STAGING_STARTER_APP_ID"
	StagingStarterKey         = "STAGING_STARTER_KEY"
	StagingEnterpriseAppID    = "STAGING_ENTERPRISE_APP_ID"
	StagingEnterpriseKey      = "STAGING_ENTERPRISE_KEY"

	PlanTypeStarter    PlanType = "starter"
	PlanTypeEnterprise PlanType = "enterprise"

	EnvProd    EnvType = "production"
	EnvStaging EnvType = "staging"

	envPath = ".env.relayutil"
)

var (
	homeDir, _ = os.UserHomeDir()
	EnvPath    = filepath.Join(homeDir, envPath)
)

type (
	EnvType  string
	PlanType string

	PortalAppData struct {
		ID  string
		Key string
	}
)

// GatherAppIDs gathers the app IDs from the environment
func GatherAppIDs() map[EnvType]map[PlanType]PortalAppData {
	return map[EnvType]map[PlanType]PortalAppData{
		EnvProd: {
			PlanTypeStarter: PortalAppData{
				ID:  mustGetString(ProductionStarterAppID),
				Key: getString(ProductionStarterKey, ""),
			},
			PlanTypeEnterprise: PortalAppData{
				ID:  mustGetString(ProductionEnterpriseAppID),
				Key: getString(ProductionEnterpriseKey, ""),
			},
		},
		EnvStaging: {
			PlanTypeStarter: PortalAppData{
				ID:  mustGetString(StagingStarterAppID),
				Key: getString(StagingStarterKey, ""),
			},
			PlanTypeEnterprise: PortalAppData{
				ID:  mustGetString(StagingEnterpriseAppID),
				Key: getString(StagingEnterpriseKey, ""),
			},
		},
	}
}

// mustGetString gets the required environment var as a string and panics if it is not present
func mustGetString(varName string) string {
	val, _ := os.LookupEnv(varName)
	if val == "" {
		panic(fmt.Sprintf("environment error (string): required env var %s not found", varName))
	}

	return val
}

// getString gets the environment var as a string
func getString(varName string, defaultValue string) string {
	val, _ := os.LookupEnv(varName)
	if val == "" {
		return defaultValue
	}

	return val
}
