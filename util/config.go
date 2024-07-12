package util

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	json5 "github.com/yosuke-furukawa/json5/encoding/json5"
)

// LoadHomeDirConfig loads a JSON5 configuration file from the user's home directory
// and unmarshals it into the provided pointer.
//
// Parameters:
// - baseFilename: The base name of the configuration file (e.g., "app_config.json5")
// - config: A pointer to a struct where the configuration will be unmarshaled
//
// Returns an error if any step fails.
// -------------------------------------------------------------------------------------
func LoadHomeDirConfig(baseFilename string, config interface{}) error {
	//---------------------------------------------
	// Get the user's home directory
	//---------------------------------------------
	home, err := homedir.Dir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	//---------------------------------------------
	// Construct the full path to the config file
	//---------------------------------------------
	configPath := filepath.Join(home, baseFilename)

	//---------------------------------------------
	// Open the file
	//---------------------------------------------
	file, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	//---------------------------------------------
	// Create a JSON5 decoder
	//---------------------------------------------
	decoder := json5.NewDecoder(file)

	//---------------------------------------------
	// Decode into the provided config struct
	//---------------------------------------------
	if err := decoder.Decode(config); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	return nil
}
