package kit

import (
	"context"
	"errors"
	"fmt"

	"github.com/mark3labs/kit/internal/models"
)

// ErrUnsupportedModelMode means that the active model does not support a mode.
var ErrUnsupportedModelMode = errors.New("unsupported model mode")

// ModelMode describes a mode available for the active model.
type ModelMode struct {
	Name              string
	Experimental      bool
	Supported         bool
	UnsupportedReason string
	// InputCost and OutputCost are estimated USD rates per million tokens.
	InputCost  float64
	OutputCost float64
}

// ModelModes returns catalog modes, including unavailable modes and their reasons.
func (m *Kit) ModelModes() []ModelMode {
	info := models.LookupModelForSettings(m.modelString)
	if info == nil {
		return nil
	}
	result := make([]ModelMode, 0, len(info.Modes))
	for _, mode := range info.Modes {
		reason := models.ModeUnsupportedReason(m.modelString, mode.Name, info)
		cost := info.EffectiveCost(mode.Name)
		result = append(result, ModelMode{Name: mode.Name, Experimental: true, Supported: reason == "", UnsupportedReason: reason, InputCost: cost.Input, OutputCost: cost.Output})
	}
	return result
}

// SupportedModelModes returns the modes supported by the active model in stable order.
func (m *Kit) SupportedModelModes() []ModelMode {
	var result []ModelMode
	for _, mode := range m.ModelModes() {
		if mode.Supported {
			result = append(result, mode)
		}
	}
	return result
}

// CurrentModelMode returns the selected mode. An empty string means the model default.
func (m *Kit) CurrentModelMode() string {
	m.promptOptsMu.RLock()
	defer m.promptOptsMu.RUnlock()
	return m.modelMode
}

// SetModelMode selects a mode for the active model. It returns an error when the
// model does not support the mode. The empty string selects the model default.
func (m *Kit) SetModelMode(ctx context.Context, mode string) error {
	m.promptOptsMu.Lock()
	defer m.promptOptsMu.Unlock()
	m.steerMu.Lock()
	active := m.steerCh != nil
	m.steerMu.Unlock()
	if active {
		return errors.New("cannot change model mode during an active turn")
	}
	if mode != "" {
		info := models.LookupModelForSettings(m.modelString)
		if info == nil {
			return fmt.Errorf("%w: %s", ErrUnsupportedModelMode, mode)
		}
		if reason := models.ModeUnsupportedReason(m.modelString, mode, info); reason != "" {
			return fmt.Errorf("%w: %s", ErrUnsupportedModelMode, reason)
		}
		descriptor, _ := info.Mode(mode)
		mode = descriptor.Name
	}
	previous := m.modelMode
	m.modelMode = mode
	if err := m.setModelLocked(ctx, m.modelString); err != nil {
		m.modelMode = previous
		return err
	}
	return nil
}
