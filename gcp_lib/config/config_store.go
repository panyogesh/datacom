// Package config provides functionality for loading and managing application configuration.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"gcp_lib/common"
	"os"
)

// Config represents the application configuration structure.
type InputParams struct {
	ServiceAccountPath string         `json:"serviceAccountPath"`
}

// ServiceAccount contains GCP service account details.
type ServiceAccount struct {
	ProjectID string `json:"project_id"`
}

type Config struct {
	// Add ServiceAccount
	SA ServiceAccount
}

const (
	// DefaultConfigPath is the default path to the configuration file.
	DefaultConfigPath = "conf/inputs/config.json"
)

var (
	// ErrConfigRead is returned when there's an error reading the config file.
	ErrConfigRead = errors.New("failed to read config file")
	// ErrConfigParse is returned when there's an error parsing the config file.
	ErrConfigParse = errors.New("failed to parse config")
)

// LoadServiceAccountDetails loads and returns the service account details from the specified path.
// Returns the service account details or an error if the operation fails.
func LoadServiceAccountDetails(serviceAccountPath string) (*ServiceAccount, error) {
	if serviceAccountPath == "" {
		return nil, errors.New("service account path cannot be empty")
	}

	cfg, err := loadConfigFile(serviceAccountPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load service account: %w", err)
	}

	return &cfg.SA, nil
}

// LoadConfig loads the application configuration from the specified path.
// If ConfigPath is empty, it uses the default configuration path.
// Returns the loaded configuration or an error if the operation fails.
func LoadConfig(configPath string) (*Config, common.ReturnStatus) {
	if configPath == "" {
		configPath = DefaultConfigPath
	}

	inputCfg, err := loadInputParamsFile(configPath)
	if err != nil {
		return nil, common.PlatformSpecific
	}

	saCfg, err := loadServicAccountConfigFile(inputCfg.ServiceAccountPath)
	if err != nil {
		return nil, common.BoilerPlateFailure
	}

	cfg := Config {
		// Update the config
		SA: saCFG
	}
	return &cfg, common.Success
}

// loadInputParamsFile is a helper function that loads and parses a configuration file.
func loadInputParamsFile(path string) (Config, error) {
	var cfg Config

	fmt.Println("[CFG] Reading config from " + path)
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("%w: %v", ErrConfigRead, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("%w: %v", ErrConfigParse, err)
	}

	return cfg, nil
}

func loadServicAccountConfigFile(path string) (ServiceAccount, error) {
	var cfg Config

	fmt.Println("[CFG] Reading config from " + path)
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("%w: %v", ErrConfigRead, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("%w: %v", ErrConfigParse, err)
	}

	return cfg, nil
}

// loadConfigFile is a helper function that loads and parses a configuration file.
func loadConfigFile(path string) (Config, error) {
	var cfg Config

	fmt.Println("[CFG] Reading config from " + path)	
	saData, err := loadServicAccountConfigFile()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("%w: %v", ErrConfigRead, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("%w: %v", ErrConfigParse, err)
	}

	return cfg, nil
}
