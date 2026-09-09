package models

import (
	"context"

	"charm.land/fantasy"
)

// UnavailableModel is a placeholder fantasy.LanguageModel used when the
// configured provider cannot be created because no credentials are
// available. It lets the agent (and the interactive TUI built on top of it)
// start normally; every generation call returns the original creation error
// until the agent is rebuilt with a real provider via SetModel.
type UnavailableModel struct {
	provider string
	model    string
	err      error
}

// NewUnavailableModel builds a placeholder model for provider/model that
// fails every call with err.
func NewUnavailableModel(provider, model string, err error) *UnavailableModel {
	return &UnavailableModel{provider: provider, model: model, err: err}
}

// Err returns the error that made the real provider unavailable.
func (u *UnavailableModel) Err() error { return u.err }

// Generate implements fantasy.LanguageModel.
func (u *UnavailableModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, u.err
}

// Stream implements fantasy.LanguageModel.
func (u *UnavailableModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	return nil, u.err
}

// GenerateObject implements fantasy.LanguageModel.
func (u *UnavailableModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, u.err
}

// StreamObject implements fantasy.LanguageModel.
func (u *UnavailableModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, u.err
}

// Provider implements fantasy.LanguageModel.
func (u *UnavailableModel) Provider() string { return u.provider }

// Model implements fantasy.LanguageModel.
func (u *UnavailableModel) Model() string { return u.model }
