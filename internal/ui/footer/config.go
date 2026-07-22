package footer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mark3labs/kit/internal/ui/prefs"
)

// Available footer field constants
const (
	FieldMode    = "mode"
	FieldModel   = "model"
	FieldContext = "context"
	FieldBar     = "bar"
	FieldCache   = "cache"
	FieldCost    = "cost"
	FieldClock   = "clock"
	FieldTimer   = "timer"
)

// AllFields contains the complete canonical list of supported footer fields in display order.
var AllFields = []string{
	FieldMode,
	FieldModel,
	FieldContext,
	FieldBar,
	FieldCache,
	FieldCost,
	FieldClock,
	FieldTimer,
}

// Config controls footer rendering behavior and field selections.
type Config struct {
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Fields  []string `yaml:"fields" json:"fields"`

	initialized bool
}

// DefaultConfig returns a Config with footer enabled and all fields selected.
func DefaultConfig() Config {
	return Config{
		Enabled:     true,
		Fields:      append([]string(nil), AllFields...),
		initialized: true,
	}
}

// LoadConfig loads persisted footer configuration from user preferences, falling back to defaults.
func LoadConfig() Config {
	cfg := DefaultConfig()
	enabled, fields := prefs.LoadFooterPreference()
	if enabled != nil {
		cfg.Enabled = *enabled
	}
	if fields != nil {
		cfg.SetFields(*fields)
	}
	return cfg
}

// SaveConfig persists the current Config to user preferences.
func SaveConfig(cfg Config) error {
	enabled := cfg.Enabled
	if err := prefs.SaveFooterPreference(&enabled, cfg.Fields); err != nil {
		return fmt.Errorf("save footer preferences: %w", err)
	}
	return nil
}

// IsFieldValid checks whether the given field name is recognized.
func IsFieldValid(field string) bool {
	return slices.Contains(AllFields, strings.ToLower(strings.TrimSpace(field)))
}

// IsFieldEnabled returns true if footer is enabled and the field is active.
func (c *Config) IsFieldEnabled(field string) bool {
	if !c.Enabled {
		return false
	}
	f := strings.ToLower(strings.TrimSpace(field))
	return slices.Contains(c.Fields, f)
}

// SetEnabled enables or disables footer display.
func (c *Config) SetEnabled(enabled bool) {
	c.Enabled = enabled
}

// ToggleEnabled toggles overall footer visibility and returns the new state.
func (c *Config) ToggleEnabled() bool {
	c.Enabled = !c.Enabled
	return c.Enabled
}

// SetFields sets the exact list of active fields, ignoring invalid duplicates.
func (c *Config) SetFields(fields []string) {
	c.initialized = true
	var valid []string
	for _, f := range fields {
		fNorm := strings.ToLower(strings.TrimSpace(f))
		if IsFieldValid(fNorm) && !slices.Contains(valid, fNorm) {
			valid = append(valid, fNorm)
		}
	}
	c.Fields = valid
}

// ToggleField toggles inclusion of a field. Returns (nowEnabled, isValid).
func (c *Config) ToggleField(field string) (bool, bool) {
	fNorm := strings.ToLower(strings.TrimSpace(field))
	if !IsFieldValid(fNorm) {
		return false, false
	}

	idx := slices.Index(c.Fields, fNorm)
	if idx >= 0 {
		c.Fields = append(c.Fields[:idx], c.Fields[idx+1:]...)
		return false, true
	}

	// Re-insert maintaining canonical AllFields order
	var newFields []string
	for _, canonical := range AllFields {
		if canonical == fNorm || slices.Contains(c.Fields, canonical) {
			newFields = append(newFields, canonical)
		}
	}
	c.Fields = newFields
	return true, true
}

// EnableField adds a field if valid and not already enabled.
func (c *Config) EnableField(field string) bool {
	fNorm := strings.ToLower(strings.TrimSpace(field))
	if !IsFieldValid(fNorm) {
		return false
	}
	if !slices.Contains(c.Fields, fNorm) {
		c.ToggleField(fNorm)
	}
	return true
}

// DisableField removes a field if present.
func (c *Config) DisableField(field string) bool {
	fNorm := strings.ToLower(strings.TrimSpace(field))
	if !IsFieldValid(fNorm) {
		return false
	}
	if idx := slices.Index(c.Fields, fNorm); idx >= 0 {
		c.Fields = append(c.Fields[:idx], c.Fields[idx+1:]...)
	}
	return true
}

// Reset restores Config to factory defaults.
func (c *Config) Reset() {
	*c = DefaultConfig()
}

// StatusMessage formats a user-friendly overview of current footer settings.
func (c *Config) StatusMessage() string {
	state := "ENABLED"
	if !c.Enabled {
		state = "DISABLED"
	}

	active := strings.Join(c.Fields, ", ")
	if len(c.Fields) == 0 {
		active = "(none)"
	}

	return fmt.Sprintf(`Footer Status: %s
Active Fields: %s
Available Fields: %s

Usage:
  /footer on|off          Enable or disable footer
  /footer toggle          Toggle footer visibility
  /footer fields <f1,f2> Set displayed fields (e.g. /footer fields mode,model,cost)
  /footer show <field>    Show specific field
  /footer hide <field>    Hide specific field
  /footer toggle <field>  Toggle specific field
  /footer reset           Reset to default settings`, state, active, strings.Join(AllFields, ", "))
}
