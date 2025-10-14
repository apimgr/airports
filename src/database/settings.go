package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Setting represents a configuration setting
type Setting struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Type        string    `json:"type"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GetSetting retrieves a setting by key
func GetSetting(key string) (*Setting, error) {
	setting := &Setting{}
	err := DB.QueryRow(`
		SELECT key, value, type, category, description, updated_at
		FROM settings WHERE key = ?
	`, key).Scan(&setting.Key, &setting.Value, &setting.Type, &setting.Category, &setting.Description, &setting.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("setting not found: %s", key)
	}
	if err != nil {
		return nil, err
	}

	return setting, nil
}

// GetSettingValue retrieves just the value as a string
func GetSettingValue(key, defaultValue string) string {
	setting, err := GetSetting(key)
	if err != nil {
		return defaultValue
	}
	return setting.Value
}

// GetSettingInt retrieves a setting as an integer
func GetSettingInt(key string, defaultValue int) int {
	setting, err := GetSetting(key)
	if err != nil {
		return defaultValue
	}
	val, err := strconv.Atoi(setting.Value)
	if err != nil {
		return defaultValue
	}
	return val
}

// GetSettingBool retrieves a setting as a boolean
func GetSettingBool(key string, defaultValue bool) bool {
	setting, err := GetSetting(key)
	if err != nil {
		return defaultValue
	}
	val, err := strconv.ParseBool(setting.Value)
	if err != nil {
		return defaultValue
	}
	return val
}

// GetSettingsByCategory retrieves all settings in a category
func GetSettingsByCategory(category string) ([]*Setting, error) {
	rows, err := DB.Query(`
		SELECT key, value, type, category, description, updated_at
		FROM settings WHERE category = ?
		ORDER BY key
	`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var settings []*Setting
	for rows.Next() {
		s := &Setting{}
		if err := rows.Scan(&s.Key, &s.Value, &s.Type, &s.Category, &s.Description, &s.UpdatedAt); err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}

	return settings, rows.Err()
}

// GetAllSettings retrieves all settings grouped by category
func GetAllSettings() (map[string][]*Setting, error) {
	rows, err := DB.Query(`
		SELECT key, value, type, category, description, updated_at
		FROM settings
		ORDER BY category, key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string][]*Setting)
	for rows.Next() {
		s := &Setting{}
		if err := rows.Scan(&s.Key, &s.Value, &s.Type, &s.Category, &s.Description, &s.UpdatedAt); err != nil {
			return nil, err
		}

		settings[s.Category] = append(settings[s.Category], s)
	}

	return settings, rows.Err()
}

// SetSetting updates or creates a setting
func SetSetting(key, value, settingType, category, description string) error {
	// Validate type
	validTypes := map[string]bool{"string": true, "number": true, "boolean": true, "json": true}
	if !validTypes[settingType] {
		return fmt.Errorf("invalid setting type: %s", settingType)
	}

	// Validate value based on type
	switch settingType {
	case "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("invalid number value: %s", value)
		}
	case "boolean":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("invalid boolean value: %s", value)
		}
	case "json":
		var js interface{}
		if err := json.Unmarshal([]byte(value), &js); err != nil {
			return fmt.Errorf("invalid JSON value: %s", value)
		}
	}

	_, err := DB.Exec(`
		INSERT INTO settings (key, value, type, category, description, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			type = excluded.type,
			category = excluded.category,
			description = excluded.description,
			updated_at = CURRENT_TIMESTAMP
	`, key, value, settingType, category, description)

	return err
}

// DeleteSetting removes a setting
func DeleteSetting(key string) error {
	_, err := DB.Exec("DELETE FROM settings WHERE key = ?", key)
	return err
}

// ResetToDefaults resets all settings to default values
func ResetToDefaults() error {
	// Delete all settings
	if _, err := DB.Exec("DELETE FROM settings"); err != nil {
		return err
	}

	// Re-run schema to insert defaults
	if _, err := DB.Exec(schemaSQL); err != nil {
		return err
	}

	return nil
}
