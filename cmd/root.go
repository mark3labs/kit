package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	charmlog "github.com/charmbracelet/log"
	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/daemon"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/ui"
	"github.com/mark3labs/kit/internal/ui/commands"
	"github.com/mark3labs/kit/internal/ui/termgfx"
	kit "github.com/mark3labs/kit/pkg/kit"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var (
	configFile        string
	systemPromptFile  string
	modelFlag         string
	decisionModelFlag string
	providerURL       string
	providerAPIKey    string
	providerWire      string
	debugMode         bool
	positionalPrompt  string        // set by processPositionalArgs from CLI positional args
	positionalFiles   []ui.FilePart // binary @file parts from processPositionalArgs

	quietFlag          bool
	jsonFlag           bool
	noExitFlag         bool
	maxSteps           int
	maxSubagents       int
	maxNestedSubagents int
	streamFlag         bool // Enable streaming output
	reduceMotion       bool // Turn off TUI transitions and layout animation
	autoCompactFlag    bool // Enable auto-compaction near context limit

	// Session management
	sessionPath             string
	capturedDaemonSessionID string

	// Tree session management (pi-style)
	continueFlag  bool // --continue / -c: resume most recent session for cwd
	resumeFlag    bool // --resume / -r: interactive session picker
	noSessionFlag bool // --no-session: ephemeral mode, no persistence

	// Model generation parameters
	maxTokens        int
	temperature      float32
	topP             float32
	topK             int32
	frequencyPenalty float32
	presencePenalty  float32
	stopSequences    []string
	thinkingLevel    string
	modelMode        string

	// Ollama-specific parameters
	numGPU  int32
	mainGPU int32

	// Extensions control
	noExtensionsFlag     bool
	noCoreToolsFlag      bool
	codeModeFlag         bool
	includeCoreToolsFlag []string
	excludeCoreToolsFlag []string
	extensionPaths       []string
	mcpFlags             []string

	// Shell tool
	shellFlag string

	// Skills control
	noSkillsFlag  bool
	skillsPaths   []string
	skillsDir     string
	skillsDisable []string

	// Named agents control
	noAgentsFlag bool

	// Bare mode — no automatic context discovery from any directory.
	bareFlag bool

	// TLS configuration
	tlsSkipVerify bool

	// Prompt templates
	promptTemplatePaths []string
	noPromptTemplates   bool

	// The daemon's directory picker
	// (--pick-dir, hidden — spawned by `kit daemon`).
	pickDirFlag bool

	// Detachable sessions. --daemon-session (hidden) marks a process the
	// daemon spawned to BE a session, so it never tries to route itself
	// into another one; --no-daemon opts one invocation out of routing.
	daemonSessionFlag bool
	noDaemonFlag      bool

	// Preference restoration flags — set in RunE after cobra parses, used
	// in runNormalMode to decide whether to apply saved preferences.
	modelFlagChanged    bool
	thinkingFlagChanged bool
)
