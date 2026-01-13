package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type UIConfig struct {
	ThemeName    string       `json:"theme_name"`
	LayoutConfig LayoutConfig `json:"layout_config"`
}

func GetDefaultThemeName() string {
	return "Dracula"
}

func DefaultUIConfig() UIConfig {
	return UIConfig{
		ThemeName:    GetDefaultThemeName(),
		LayoutConfig: DefaultLayoutConfig(),
	}
}

func (c *UIConfig) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func LoadUIConfig(path string) (UIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultUIConfig(), nil
		}
		return UIConfig{}, fmt.Errorf("failed to read config: %w", err)
	}

	var config UIConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return UIConfig{}, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return config, nil
}
