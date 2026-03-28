//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"kit/ext"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────

type RelayConfig struct {
	Version         int     `json:"version"`
	Enabled         bool    `json:"enabled"`
	BotToken        string  `json:"botToken"`
	BotID           int64   `json:"botId"`
	BotUsername     string  `json:"botUsername"`
	ChatID          int64   `json:"chatId"`
	AllowedUserIDs  []int64 `json:"allowedUserIds"`
	LastValidatedAt string  `json:"lastValidatedAt"`
}

type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

type TelegramChat struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Username string `json:"username"`
}

type TelegramChatMember struct {
	Status string       `json:"status"`
	User   TelegramUser `json:"user"`
}

type TelegramMessage struct {
	MessageID int          `json:"message_id"`
	Date      int64        `json:"date"`
	Text      string       `json:"text"`
	Caption   string       `json:"caption"`
	From      TelegramUser `json:"from"`
	Chat      TelegramChat `json:"chat"`
	EditDate  int64        `json:"edit_date"`
}

type TelegramUpdate struct {
	UpdateID      int64            `json:"update_id"`
	Message       *TelegramMessage `json:"message"`
	EditedMessage *TelegramMessage `json:"edited_message"`
}

type TelegramEnvelope struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
}

type QueuedInput struct {
	Seq               int
	TelegramMessageID int
	SenderID          int64
	Text              string
	AcceptedAt        time.Time
	Dispatched        bool
}

type RenderAction struct {
	ID     string
	Label  string
	Status string // "running", "done", "error"
}

type ActiveRunState struct {
	ID                 int
	StartedAt          time.Time
	StepCount          int
	ProgressMessageID  int
	LastRenderedText   string
	Actions            []RenderAction
	LastAssistantText  string
	LastAssistantError bool
}

type PendingTest struct {
	Code      string
	MessageID int
	ExpiresAt time.Time
}

// ──────────────────────────────────────────────
// Constants
// ──────────────────────────────────────────────

const (
	telegramAPIBase         = "https://api.telegram.org"
	statusKey               = "kit-telegram"
	retryIntervalMS         = 5000
	normalPollTimeoutS      = 50
	warmupPollTimeoutS      = 0
	clientTimeoutBufferMS   = 15000
	minProgressEditInterval = 2 * time.Second
	testTimeoutMS           = 60000
	chatDiscoveryTimeoutMS  = 60000
	chatDiscoveryPollS      = 10
	maxProgressActions      = 5
	maxBodyChars            = 3500
)

// ──────────────────────────────────────────────
// Package-level state (Yaegi-compatible)
// ──────────────────────────────────────────────

var (
	mu sync.Mutex

	// Config
	config *RelayConfig

	// Relay connection
	pollLoopActive    bool
	pollGeneration    int
	pollStopCh        chan struct{}
	lastAPISuccessAt  time.Time
	retryActive       bool
	retryAttempt      int
	retryLogPath      string
	currentOffset     int64
	offsetInitialized bool
	isConnecting      bool

	// Spinner
	spinnerIndex  int
	spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	healthTicker  *time.Ticker
	healthStop    chan struct{}

	// Queue
	queue      []*QueuedInput
	queueIndex map[int]*QueuedInput // telegram message_id -> queued item
	nextSeq    int

	// Run state
	activeRun          *ActiveRunState
	nextRunID          int
	agentBusy          bool
	lastProgressEditAt time.Time

	// Test
	pendingTest *PendingTest

	// Latest context for background goroutines
	latestCtx    ext.Context
	latestCtxSet bool

	// Debug mode
	debugMode bool

	// Project directory (set from ctx.CWD)
	projectDir string
)

// ──────────────────────────────────────────────
// Debug logging
// ──────────────────────────────────────────────

func report(kind string, msg string) {
	if !debugMode {
		return
	}
	fmt.Printf("[kit-telegram] %s: %s\n", kind, msg)
}

// ──────────────────────────────────────────────
// Config file management
// ──────────────────────────────────────────────

func configDir() string {
	if projectDir != "" {
		return filepath.Join(projectDir, ".kit")
	}
	// Fallback if projectDir not yet set (should not happen in normal flow)
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "kit")
}

func configPath() string {
	return filepath.Join(configDir(), "kit-telegram.json")
}

func failureLogDir() string {
	return filepath.Join(configDir(), "kit-telegram")
}

func readRelayConfig() (*RelayConfig, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cfg RelayConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported config version: %d", cfg.Version)
	}
	return &cfg, nil
}

func writeRelayConfig(cfg *RelayConfig) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := fmt.Sprintf("%s.tmp-%d-%d", configPath(), os.Getpid(), time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, append(data, '\n'), 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, configPath())
}

func deleteRelayConfig() error {
	return os.Remove(configPath())
}

func createRetryLogPath() string {
	now := time.Now()
	stamp := now.Format("20060102-150405")
	return filepath.Join(failureLogDir(), stamp+".log")
}

func appendFailureLog(path string, entry map[string]any) {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	data, _ := json.Marshal(entry)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}

// ──────────────────────────────────────────────
// Telegram Bot API client
// ──────────────────────────────────────────────

func telegramRequest(token string, method string, body map[string]any, timeoutSec int) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/bot%s/%s", telegramAPIBase, token, method)
	payload, _ := json.Marshal(body)
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("network error: %s", err.Error())
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read error: %s", err.Error())
	}
	var envelope TelegramEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("JSON parse error: %s", err.Error())
	}
	if !envelope.OK {
		return nil, fmt.Errorf("Telegram API error %d: %s", envelope.ErrorCode, envelope.Description)
	}
	return envelope.Result, nil
}

func tgGetMe(token string) (*TelegramUser, error) {
	result, err := telegramRequest(token, "getMe", map[string]any{}, 15)
	if err != nil {
		return nil, err
	}
	var user TelegramUser
	if err := json.Unmarshal(result, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func tgGetChat(token string, chatID int64) (*TelegramChat, error) {
	result, err := telegramRequest(token, "getChat", map[string]any{
		"chat_id": chatID,
	}, 15)
	if err != nil {
		return nil, err
	}
	var chat TelegramChat
	if err := json.Unmarshal(result, &chat); err != nil {
		return nil, err
	}
	return &chat, nil
}

func tgGetChatMember(token string, chatID int64, userID int64) (*TelegramChatMember, error) {
	result, err := telegramRequest(token, "getChatMember", map[string]any{
		"chat_id": chatID,
		"user_id": userID,
	}, 15)
	if err != nil {
		return nil, err
	}
	var member TelegramChatMember
	if err := json.Unmarshal(result, &member); err != nil {
		return nil, err
	}
	return &member, nil
}

func tgGetUpdates(token string, offset int64, hasOffset bool, timeoutSeconds int, clientTimeoutSec int) ([]TelegramUpdate, error) {
	body := map[string]any{
		"timeout":         timeoutSeconds,
		"allowed_updates": []string{"message", "edited_message"},
	}
	if hasOffset {
		body["offset"] = offset
	}
	result, err := telegramRequest(token, "getUpdates", body, clientTimeoutSec)
	if err != nil {
		return nil, err
	}
	var updates []TelegramUpdate
	if err := json.Unmarshal(result, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func tgSendMessage(token string, chatID int64, text string) (*TelegramMessage, error) {
	result, err := telegramRequest(token, "sendMessage", map[string]any{
		"chat_id":                  chatID,
		"text":                     text,
		"disable_web_page_preview": true,
	}, 30)
	if err != nil {
		return nil, err
	}
	var msg TelegramMessage
	if err := json.Unmarshal(result, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func tgEditMessageText(token string, chatID int64, messageID int, text string) (*TelegramMessage, error) {
	result, err := telegramRequest(token, "editMessageText", map[string]any{
		"chat_id":                  chatID,
		"message_id":               messageID,
		"text":                     text,
		"disable_web_page_preview": true,
	}, 30)
	if err != nil {
		return nil, err
	}
	var msg TelegramMessage
	if err := json.Unmarshal(result, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ──────────────────────────────────────────────
// Error classification
// ──────────────────────────────────────────────

func isConnectionAffecting(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	// Non-connection-affecting message errors
	if strings.Contains(msg, "message is not modified") ||
		strings.Contains(msg, "message can't be edited") ||
		strings.Contains(msg, "message to edit not found") {
		return false
	}
	// Connection-affecting errors
	if strings.Contains(msg, "network") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "401") ||
		strings.Contains(msg, "409") ||
		strings.Contains(msg, "429") ||
		strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "fetch") {
		return true
	}
	// Default: assume connection-affecting for safety
	return true
}

// ──────────────────────────────────────────────
// Relay helpers: send/edit to Telegram
// ──────────────────────────────────────────────

func telegramSend(text string) int {
	mu.Lock()
	cfg := config
	mu.Unlock()
	if cfg == nil {
		return 0
	}
	msg, err := tgSendMessage(cfg.BotToken, cfg.ChatID, text)
	if err != nil {
		handleAPIFailure(err, "sendMessage")
		return 0
	}
	recordAPISuccess()
	return msg.MessageID
}

func telegramEdit(messageID int, text string) bool {
	mu.Lock()
	cfg := config
	mu.Unlock()
	if cfg == nil {
		return false
	}
	_, err := tgEditMessageText(cfg.BotToken, cfg.ChatID, messageID, text)
	if err != nil {
		handleAPIFailure(err, "editMessageText")
		return false
	}
	recordAPISuccess()
	return true
}

func recordAPISuccess() {
	mu.Lock()
	lastAPISuccessAt = time.Now()
	retryActive = false
	retryAttempt = 0
	retryLogPath = ""
	mu.Unlock()
	refreshFooter()
}

func handleAPIFailure(err error, operation string) {
	if !isConnectionAffecting(err) {
		report("api.failure.non_health", fmt.Sprintf("%s: %s", operation, err.Error()))
		return
	}
	mu.Lock()
	retryActive = true
	retryAttempt++
	if retryLogPath == "" {
		retryLogPath = createRetryLogPath()
	}
	logPath := retryLogPath
	attempt := retryAttempt
	mu.Unlock()

	appendFailureLog(logPath, map[string]any{
		"timestamp":     time.Now().Format(time.RFC3339),
		"operation":     operation,
		"attempt":       attempt,
		"error_type":    "api_error",
		"error_message": err.Error(),
	})
	report("api.failure", fmt.Sprintf("%s attempt=%d: %s", operation, attempt, err.Error()))
	refreshFooter()
}

// ──────────────────────────────────────────────
// Connection state
// ──────────────────────────────────────────────

func isConnected() bool {
	mu.Lock()
	defer mu.Unlock()
	if config == nil || !config.Enabled {
		return false
	}
	if !pollLoopActive {
		return false
	}
	if lastAPISuccessAt.IsZero() {
		return false
	}
	if retryActive {
		return false
	}
	return true
}

func botRef() string {
	mu.Lock()
	defer mu.Unlock()
	if config != nil && config.BotUsername != "" {
		return "@" + config.BotUsername
	}
	return "bot"
}

// ──────────────────────────────────────────────
// Footer / Status bar
// ──────────────────────────────────────────────

func refreshFooter() {
	mu.Lock()
	ctxSet := latestCtxSet
	ctx := latestCtx
	mu.Unlock()
	if !ctxSet {
		return
	}
	var text string
	if isConnected() {
		text = "Telegram Connected · " + botRef()
		mu.Lock()
		qLen := len(queue)
		mu.Unlock()
		if qLen > 0 {
			text = fmt.Sprintf("%s · %d queued", text, qLen)
		}
	} else {
		mu.Lock()
		connecting := isConnecting
		retry := retryActive
		idx := spinnerIndex % len(spinnerFrames)
		mu.Unlock()
		if connecting {
			text = spinnerFrames[idx] + " Telegram Connecting"
		} else if retry {
			text = fmt.Sprintf("Telegram Disconnected · retrying in %ds", retryIntervalMS/1000)
		} else {
			text = "Telegram Disconnected"
		}
	}
	ctx.SetStatus(statusKey, text, 20)
}

func clearFooter() {
	mu.Lock()
	ctxSet := latestCtxSet
	ctx := latestCtx
	mu.Unlock()
	if !ctxSet {
		return
	}
	ctx.RemoveStatus(statusKey)
}

// ──────────────────────────────────────────────
// Working message widget
// ──────────────────────────────────────────────

func setWorkingMessage(msg string) {
	mu.Lock()
	ctxSet := latestCtxSet
	ctx := latestCtx
	mu.Unlock()
	if !ctxSet {
		return
	}
	if msg == "" {
		ctx.RemoveWidget("telegram-working")
		return
	}
	ctx.SetWidget(ext.WidgetConfig{
		ID:        "telegram-working",
		Placement: ext.WidgetAbove,
		Content:   ext.WidgetContent{Text: msg},
		Style:     ext.WidgetStyle{BorderColor: "#89b4fa"},
		Priority:  5,
	})
}

// ──────────────────────────────────────────────
// Health timer (spinner + test expiry)
// ──────────────────────────────────────────────

func ensureHealthTimer() {
	mu.Lock()
	defer mu.Unlock()
	if healthTicker != nil {
		return
	}
	healthTicker = time.NewTicker(200 * time.Millisecond)
	healthStop = make(chan struct{})
	go func() {
		for {
			select {
			case <-healthTicker.C:
				expirePendingTest()
				mu.Lock()
				connecting := isConnecting
				if connecting {
					spinnerIndex++
				}
				mu.Unlock()
				refreshFooter()
			case <-healthStop:
				return
			}
		}
	}()
}

func clearHealthTimer() {
	mu.Lock()
	defer mu.Unlock()
	if healthTicker != nil {
		healthTicker.Stop()
		close(healthStop)
		healthTicker = nil
	}
}

// ──────────────────────────────────────────────
// Polling lifecycle
// ──────────────────────────────────────────────

func startPolling() {
	mu.Lock()
	if pollLoopActive {
		mu.Unlock()
		return
	}
	if config == nil || !config.Enabled {
		mu.Unlock()
		return
	}
	pollGeneration++
	gen := pollGeneration
	pollLoopActive = true
	retryActive = false
	retryAttempt = 0
	retryLogPath = ""
	isConnecting = true
	pollStopCh = make(chan struct{})
	stopCh := pollStopCh
	token := config.BotToken
	mu.Unlock()

	refreshFooter()

	go func() {
		pollLoop(gen, stopCh, token)
	}()
}

func stopPolling() {
	mu.Lock()
	if !pollLoopActive {
		mu.Unlock()
		return
	}
	pollGeneration++
	pollLoopActive = false
	isConnecting = false
	if pollStopCh != nil {
		close(pollStopCh)
		pollStopCh = nil
	}
	mu.Unlock()
	refreshFooter()
}

func pollLoop(generation int, stopCh chan struct{}, token string) {
	firstPoll := true
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		mu.Lock()
		if pollGeneration != generation {
			mu.Unlock()
			return
		}
		off := currentOffset
		hasOff := offsetInitialized
		mu.Unlock()

		pollTimeoutS := normalPollTimeoutS
		if firstPoll {
			pollTimeoutS = warmupPollTimeoutS
		}
		clientTimeoutS := pollTimeoutS + (clientTimeoutBufferMS / 1000)

		updates, err := tgGetUpdates(token, off, hasOff, pollTimeoutS, clientTimeoutS)

		// Check if stopped
		select {
		case <-stopCh:
			return
		default:
		}

		mu.Lock()
		if pollGeneration != generation {
			mu.Unlock()
			return
		}
		mu.Unlock()

		if err != nil {
			handleAPIFailure(err, "getUpdates")
			// Sleep before retry
			select {
			case <-stopCh:
				return
			case <-time.After(time.Duration(retryIntervalMS) * time.Millisecond):
			}
			continue
		}

		recordAPISuccess()
		mu.Lock()
		isConnecting = false
		mu.Unlock()
		firstPoll = false

		for _, update := range updates {
			mu.Lock()
			currentOffset = update.UpdateID + 1
			offsetInitialized = true
			mu.Unlock()
			handleTelegramUpdate(update)
		}

		refreshFooter()
	}
}

func ensureDesiredConnection() {
	mu.Lock()
	cfg := config
	mu.Unlock()
	if cfg == nil || !cfg.Enabled {
		stopPolling()
		return
	}
	startPolling()
}

// ──────────────────────────────────────────────
// Queue management
// ──────────────────────────────────────────────

func initQueue() {
	queue = make([]*QueuedInput, 0)
	queueIndex = make(map[int]*QueuedInput)
	nextSeq = 1
}

func queueLength() int {
	mu.Lock()
	defer mu.Unlock()
	return len(queue)
}

func enqueue(text string, telegramMessageID int, senderID int64) *QueuedInput {
	mu.Lock()
	defer mu.Unlock()
	seq := nextSeq
	nextSeq++
	item := &QueuedInput{
		Seq:               seq,
		TelegramMessageID: telegramMessageID,
		SenderID:          senderID,
		Text:              text,
		AcceptedAt:        time.Now(),
		Dispatched:        false,
	}
	queue = append(queue, item)
	queueIndex[telegramMessageID] = item
	report("queue.accepted", fmt.Sprintf("seq=%d msgId=%d text=%s qLen=%d", seq, telegramMessageID, truncate(text, 60), len(queue)))
	return item
}

func tryEditQueued(telegramMessageID int, newText string) bool {
	mu.Lock()
	defer mu.Unlock()
	item, ok := queueIndex[telegramMessageID]
	if !ok || item.Dispatched {
		return false
	}
	item.Text = newText
	report("queue.updated", fmt.Sprintf("msgId=%d seq=%d", telegramMessageID, item.Seq))
	return true
}

func dispatchOrEnqueue(text string, telegramMessageID int, senderID int64) {
	// Try immediate dispatch
	mu.Lock()
	ctxSet := latestCtxSet
	ctx := latestCtx
	mu.Unlock()
	if ctxSet {
		ctx.SendMessage(text)
		report("dispatch.immediate", fmt.Sprintf("msgId=%d text=%s", telegramMessageID, truncate(text, 60)))
		return
	}
	enqueue(text, telegramMessageID, senderID)
}

func promoteOneToNewRun() bool {
	mu.Lock()
	if len(queue) == 0 {
		mu.Unlock()
		return false
	}
	item := queue[0]
	queue = queue[1:]
	delete(queueIndex, item.TelegramMessageID)
	text := item.Text
	ctxSet := latestCtxSet
	ctx := latestCtx
	mu.Unlock()

	if ctxSet {
		ctx.SendMessage(text)
		report("queue.promoted", fmt.Sprintf("seq=%d text=%s", item.Seq, truncate(text, 60)))
		return true
	}
	return false
}

// ──────────────────────────────────────────────
// Render functions
// ──────────────────────────────────────────────

func truncate(text string, max int) string {
	if len(text) <= max {
		return text
	}
	return text[:max-1] + "…"
}

func formatElapsed(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	if totalSeconds < 0 {
		totalSeconds = 0
	}
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	hours := minutes / 60
	remainingMinutes := minutes % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %02dm", hours, remainingMinutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %02ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func summarizeToolAction(toolName string, inputJSON string) string {
	var args map[string]any
	json.Unmarshal([]byte(inputJSON), &args)
	if args == nil {
		args = make(map[string]any)
	}
	getStr := func(key string, fallback string) string {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
		return fallback
	}
	switch toolName {
	case "read":
		return "reading " + getStr("path", "file")
	case "write":
		return "writing " + getStr("path", "file")
	case "edit":
		return "editing " + getStr("path", "file")
	case "bash":
		return "running " + truncate(getStr("command", "command"), 80)
	case "find":
		return "finding " + getStr("pattern", getStr("path", "files"))
	case "grep":
		return "searching " + getStr("pattern", "text")
	case "ls":
		return "listing " + getStr("path", "directory")
	case "subagent":
		return "spawning subagent"
	default:
		return "using " + toolName
	}
}

func summarizeToolResult(toolName string, inputJSON string, isError bool) string {
	if isError {
		return "failed " + summarizeToolAction(toolName, inputJSON)
	}
	var args map[string]any
	json.Unmarshal([]byte(inputJSON), &args)
	if args == nil {
		args = make(map[string]any)
	}
	getStr := func(key string, fallback string) string {
		if v, ok := args[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
		return fallback
	}
	switch toolName {
	case "write":
		return "updated " + getStr("path", "file")
	case "edit":
		return "edited " + getStr("path", "file")
	case "read":
		return "read " + getStr("path", "file")
	case "bash":
		return "completed " + truncate(getStr("command", "command"), 80)
	default:
		return "completed " + toolName
	}
}

func renderActionLine(action RenderAction) string {
	prefix := "↻"
	switch action.Status {
	case "done":
		prefix = "✓"
	case "error":
		prefix = "✗"
	}
	return prefix + " " + action.Label
}

func renderProgress(run *ActiveRunState) string {
	elapsed := time.Since(run.StartedAt)
	step := run.StepCount
	if step < 1 {
		step = 1
	}
	header := fmt.Sprintf("⏳ %s · step %d", formatElapsed(elapsed), step)

	actions := run.Actions
	if len(actions) > maxProgressActions {
		actions = actions[len(actions)-maxProgressActions:]
	}
	lines := make([]string, 0, len(actions))
	for _, a := range actions {
		lines = append(lines, renderActionLine(a))
	}
	body := ""
	if len(lines) > 0 {
		body = "\n\n" + strings.Join(lines, "\n")
	}
	return header + body
}

func renderFinal(run *ActiveRunState) string {
	elapsed := time.Since(run.StartedAt)
	prefix := "✅"
	if run.LastAssistantError {
		prefix = "❌"
	}
	header := fmt.Sprintf("%s %s", prefix, formatElapsed(elapsed))
	body := strings.TrimSpace(run.LastAssistantText)
	if body == "" {
		if run.LastAssistantError {
			body = "Run failed."
		} else {
			body = "Run completed."
		}
	}
	return header + "\n\n" + body
}

func splitFinalText(text string) []string {
	if len(text) <= maxBodyChars {
		return []string{text}
	}
	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	current := ""
	for _, para := range paragraphs {
		next := current
		if next == "" {
			next = para
		} else {
			next = next + "\n\n" + para
		}
		if len(next) <= maxBodyChars {
			current = next
			continue
		}
		if current != "" {
			chunks = append(chunks, current)
			current = ""
		}
		if len(para) <= maxBodyChars {
			current = para
		} else {
			// Split long paragraph by lines
			chunks = append(chunks, splitLongText(para, maxBodyChars)...)
		}
	}
	if current != "" {
		chunks = append(chunks, current)
	}
	if len(chunks) <= 1 {
		return chunks
	}
	// Label continuation chunks
	total := len(chunks)
	result := make([]string, total)
	for i, chunk := range chunks {
		if i == 0 {
			result[i] = chunk
		} else {
			result[i] = fmt.Sprintf("continued (%d/%d)\n\n%s", i+1, total, chunk)
		}
	}
	return result
}

func splitLongText(text string, maxChars int) []string {
	var chunks []string
	current := ""
	lines := strings.Split(text, "\n")
	openFence := false
	fenceHeader := "```"
	for _, line := range lines {
		next := current
		if next == "" {
			next = line
		} else {
			next = next + "\n" + line
		}
		if len(next) > maxChars && current != "" {
			flushed := current
			if openFence {
				flushed = flushed + "\n```"
			}
			chunks = append(chunks, flushed)
			if openFence {
				current = fenceHeader + "\n" + line
			} else {
				current = line
			}
		} else {
			current = next
		}
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if openFence {
				openFence = false
			} else {
				openFence = true
				fenceHeader = strings.TrimSpace(line)
			}
		}
	}
	if current != "" {
		if openFence {
			current = current + "\n```"
		}
		chunks = append(chunks, current)
	}
	return chunks
}

// ──────────────────────────────────────────────
// Progress messaging
// ──────────────────────────────────────────────

func ensureProgressMessage() {
	mu.Lock()
	run := activeRun
	cfg := config
	mu.Unlock()
	if run == nil || run.ProgressMessageID != 0 {
		return
	}
	if cfg == nil || !cfg.Enabled {
		return
	}
	if !isConnected() {
		return
	}
	text := renderProgress(run)
	msgID := telegramSend(text)
	if msgID == 0 {
		return
	}
	mu.Lock()
	if activeRun == run {
		run.ProgressMessageID = msgID
		run.LastRenderedText = text
		lastProgressEditAt = time.Now()
	}
	mu.Unlock()
	report("run.progress.created", fmt.Sprintf("runId=%d msgId=%d", run.ID, msgID))
}

func updateProgressMessage() {
	mu.Lock()
	run := activeRun
	cfg := config
	mu.Unlock()
	if run == nil || cfg == nil || !cfg.Enabled {
		return
	}

	ensureProgressMessage()

	mu.Lock()
	if activeRun != run || run.ProgressMessageID == 0 {
		mu.Unlock()
		return
	}
	now := time.Now()
	if now.Sub(lastProgressEditAt) < minProgressEditInterval {
		mu.Unlock()
		return
	}
	mu.Unlock()

	rendered := renderProgress(run)
	if rendered == run.LastRenderedText {
		return
	}

	ok := telegramEdit(run.ProgressMessageID, rendered)
	if ok {
		mu.Lock()
		run.LastRenderedText = rendered
		lastProgressEditAt = time.Now()
		mu.Unlock()
		report("run.progress.edited", fmt.Sprintf("runId=%d", run.ID))
	}
}

func finalizeRun() {
	mu.Lock()
	run := activeRun
	cfg := config
	mu.Unlock()
	if run == nil || cfg == nil || !cfg.Enabled {
		return
	}

	ensureProgressMessage()

	mu.Lock()
	if run.ProgressMessageID == 0 {
		mu.Unlock()
		return
	}
	mu.Unlock()

	finalText := renderFinal(run)
	chunks := splitFinalText(finalText)
	first := finalText
	if len(chunks) > 0 {
		first = chunks[0]
	}
	ok := telegramEdit(run.ProgressMessageID, first)
	if ok {
		report("run.finalized", fmt.Sprintf("runId=%d chunks=%d", run.ID, len(chunks)))
	}
	for i := 1; i < len(chunks); i++ {
		telegramSend(chunks[i])
	}
}

// ──────────────────────────────────────────────
// Telegram update dispatch
// ──────────────────────────────────────────────

func handleTelegramUpdate(update TelegramUpdate) {
	mu.Lock()
	cfg := config
	mu.Unlock()
	if cfg == nil {
		return
	}

	rawMsg := update.Message
	isEdit := false
	if rawMsg == nil && update.EditedMessage != nil {
		rawMsg = update.EditedMessage
		isEdit = true
	}
	if rawMsg == nil {
		return
	}

	senderID := rawMsg.From.ID
	chatID := rawMsg.Chat.ID

	// Reject wrong chat
	if chatID != cfg.ChatID {
		report("reject", fmt.Sprintf("wrong_chat chatId=%d", chatID))
		return
	}

	// Reject non-whitelisted or bot senders
	allowed := false
	for _, id := range cfg.AllowedUserIDs {
		if id == senderID {
			allowed = true
			break
		}
	}
	if !allowed || senderID == 0 || rawMsg.From.IsBot {
		report("reject", fmt.Sprintf("sender_not_allowed senderId=%d", senderID))
		return
	}

	// Test reply check
	mu.Lock()
	test := pendingTest
	mu.Unlock()
	if test != nil && time.Now().Before(test.ExpiresAt) && strings.TrimSpace(rawMsg.Text) == test.Code {
		completeTestSuccess(senderID)
		return
	}

	// Edits of queued items
	if isEdit {
		if rawMsg.Text == "" {
			report("reject", "caption_or_non_text_edit")
			return
		}
		tryEditQueued(rawMsg.MessageID, rawMsg.Text)
		return
	}

	// Only text messages count as prompts
	if rawMsg.Text == "" {
		report("reject", "caption_or_non_text")
		return
	}
	text := strings.TrimSpace(rawMsg.Text)
	if text == "" {
		return
	}

	// Remote commands
	if handleRemoteTelegramCommand(text) {
		return
	}

	// Prompt dispatch
	mu.Lock()
	busy := agentBusy
	mu.Unlock()

	if !busy {
		dispatchOrEnqueue(text, rawMsg.MessageID, senderID)
	} else {
		enqueue(text, rawMsg.MessageID, senderID)
	}
}

// ──────────────────────────────────────────────
// Remote command handling
// ──────────────────────────────────────────────

func handleRemoteTelegramCommand(text string) bool {
	if !strings.HasPrefix(text, "/telegram") {
		return false
	}
	parts := strings.Fields(text)
	sub := ""
	if len(parts) > 1 {
		sub = parts[1]
	}
	switch sub {
	case "":
		go func() { telegramSend(buildCommandHelpText()) }()
	case "status":
		go func() { telegramSend(buildStatusText()) }()
	case "test":
		go remoteTest()
	case "toggle":
		go remoteToggle()
	case "logout":
		rest := ""
		if len(parts) > 2 {
			rest = parts[2]
		}
		if rest != "yes" {
			go func() { telegramSend("Remote logout requires confirmation. Send: /telegram logout yes") }()
		} else {
			go func() {
				telegramSend("Telegram relay logging out.")
				logoutCore()
			}()
		}
	case "clear":
		go func() {
			setWorkingMessage("")
			clearFooter()
			telegramSend("Telegram TUI cleared.")
		}()
	case "connect":
		go func() { telegramSend("Remote connect is not available. Use local /telegram connect.") }()
	default:
		go func() { telegramSend(buildCommandHelpText()) }()
	}
	return true
}

// ──────────────────────────────────────────────
// Test command
// ──────────────────────────────────────────────

func runRelayTest(ctx ext.Context) {
	mu.Lock()
	cfg := config
	mu.Unlock()
	if cfg == nil {
		ctx.PrintError("No relay config found. Use /telegram connect.")
		return
	}
	code := fmt.Sprintf("%04d", rand.Intn(9000)+1000)
	text := fmt.Sprintf("Telegram relay test\n\nReply to this message with: %s\nThis check expires in 60 seconds.", code)
	setWorkingMessage("Sending Telegram test message...")
	msgID := telegramSend(text)
	if msgID == 0 {
		ctx.PrintError("Could not send the Telegram test message.")
		setWorkingMessage("")
		return
	}
	mu.Lock()
	pendingTest = &PendingTest{
		Code:      code,
		MessageID: msgID,
		ExpiresAt: time.Now().Add(time.Duration(testTimeoutMS) * time.Millisecond),
	}
	mu.Unlock()
	setWorkingMessage("Waiting for test reply in Telegram...")
	report("test.sent", fmt.Sprintf("code=%s msgId=%d", code, msgID))
}

func completeTestSuccess(senderID int64) {
	mu.Lock()
	test := pendingTest
	pendingTest = nil
	mu.Unlock()
	if test == nil {
		return
	}
	telegramEdit(test.MessageID, "Telegram relay test\n\nSuccess. Outbound and inbound relay both work.")
	report("test.success", fmt.Sprintf("senderId=%d", senderID))
	setWorkingMessage("")
}

func expirePendingTest() {
	mu.Lock()
	test := pendingTest
	if test == nil || time.Now().Before(test.ExpiresAt) {
		mu.Unlock()
		return
	}
	pendingTest = nil
	mu.Unlock()
	telegramEdit(test.MessageID, "Telegram relay test\n\nExpired. No matching reply was received in time.")
	report("test.expired", fmt.Sprintf("msgId=%d", test.MessageID))
	setWorkingMessage("")
}

func remoteTest() {
	mu.Lock()
	cfg := config
	mu.Unlock()
	if cfg == nil {
		return
	}
	code := fmt.Sprintf("%04d", rand.Intn(9000)+1000)
	body := fmt.Sprintf("Telegram relay test\n\nReply to this message with: %s\nThis check expires in 60 seconds.", code)
	msgID := telegramSend(body)
	if msgID != 0 {
		mu.Lock()
		pendingTest = &PendingTest{
			Code:      code,
			MessageID: msgID,
			ExpiresAt: time.Now().Add(time.Duration(testTimeoutMS) * time.Millisecond),
		}
		mu.Unlock()
	}
}

// ──────────────────────────────────────────────
// Toggle command
// ──────────────────────────────────────────────

func toggleRelay(ctx ext.Context) {
	mu.Lock()
	cfg := config
	mu.Unlock()
	if cfg == nil {
		ctx.PrintError("No relay config found. Use /telegram connect.")
		return
	}
	newEnabled := !cfg.Enabled
	cfg.Enabled = newEnabled

	stopPolling()
	mu.Lock()
	config = cfg
	mu.Unlock()
	writeRelayConfig(cfg)

	if newEnabled {
		ensureDesiredConnection()
		ctx.PrintInfo("Telegram relay enabled.")
	} else {
		ctx.PrintInfo("Telegram relay disabled.")
	}
	refreshFooter()
}

func remoteToggle() {
	mu.Lock()
	cfg := config
	mu.Unlock()
	if cfg == nil {
		return
	}
	if cfg.Enabled {
		telegramSend("Telegram relay disabling.")
	}
	newEnabled := !cfg.Enabled
	cfg.Enabled = newEnabled

	stopPolling()
	mu.Lock()
	config = cfg
	mu.Unlock()
	writeRelayConfig(cfg)

	if newEnabled {
		ensureDesiredConnection()
		telegramSend("Telegram relay enabled.")
	}
	refreshFooter()
}

// ──────────────────────────────────────────────
// Logout command
// ──────────────────────────────────────────────

func logoutRelay(ctx ext.Context) {
	result := ctx.PromptConfirm(ext.PromptConfirmConfig{
		Message:      "Logout Telegram relay? This removes saved credentials and disconnects.",
		DefaultValue: false,
	})
	if result.Cancelled || !result.Value {
		return
	}
	logoutCore()
	ctx.PrintInfo("Telegram relay logged out.")
}

func logoutCore() {
	stopPolling()
	deleteRelayConfig()
	mu.Lock()
	config = nil
	lastAPISuccessAt = time.Time{}
	retryActive = false
	retryAttempt = 0
	retryLogPath = ""
	pendingTest = nil
	mu.Unlock()
	refreshFooter()
}

// ──────────────────────────────────────────────
// Status / help text
// ──────────────────────────────────────────────

func buildStatusText() string {
	mu.Lock()
	cfg := config
	connRetry := retryActive
	logPath := retryLogPath
	successAt := lastAPISuccessAt
	mu.Unlock()

	conn := "disconnected"
	if isConnected() {
		conn = "connected"
	}
	enabled := "false"
	botUsername := "none"
	botID := "none"
	chatID := "none"
	allowedIDs := "none"
	if cfg != nil {
		enabled = strconv.FormatBool(cfg.Enabled)
		if cfg.BotUsername != "" {
			botUsername = cfg.BotUsername
		}
		botID = strconv.FormatInt(cfg.BotID, 10)
		chatID = strconv.FormatInt(cfg.ChatID, 10)
		ids := make([]string, len(cfg.AllowedUserIDs))
		for i, id := range cfg.AllowedUserIDs {
			ids[i] = strconv.FormatInt(id, 10)
		}
		if len(ids) > 0 {
			allowedIDs = strings.Join(ids, ",")
		}
	}
	qLen := queueLength()
	progressMsgID := "none"
	mu.Lock()
	if activeRun != nil && activeRun.ProgressMessageID != 0 {
		progressMsgID = strconv.Itoa(activeRun.ProgressMessageID)
	}
	mu.Unlock()
	lastSuccess := "none"
	if !successAt.IsZero() {
		lastSuccess = successAt.Format(time.RFC3339)
	}
	retry := "inactive"
	if connRetry {
		retry = "active"
	}
	failLog := "none"
	if logPath != "" {
		failLog = logPath
	}
	return fmt.Sprintf(`connection: %s
enabled: %s
bot_username: %s
bot_id: %s
chat_id: %s
allowed_user_ids: %s
queue_length: %d
active_progress_message_id: %s
last_api_success_at: %s
retry_state: %s
failure_log_path: %s`, conn, enabled, botUsername, botID, chatID, allowedIDs, qLen, progressMsgID, lastSuccess, retry, failLog)
}

func buildCommandHelpText() string {
	conn := "disconnected"
	if isConnected() {
		conn = "connected"
	} else {
		mu.Lock()
		connecting := isConnecting
		mu.Unlock()
		if connecting {
			conn = "connecting"
		}
	}
	mu.Lock()
	cfg := config
	mu.Unlock()
	enabled := "off"
	botLabel := "not configured"
	chatLabel := "not configured"
	allowedLabel := "not configured"
	if cfg != nil {
		if cfg.Enabled {
			enabled = "on"
		}
		botLabel = botRef()
		chatLabel = strconv.FormatInt(cfg.ChatID, 10)
		ids := make([]string, len(cfg.AllowedUserIDs))
		for i, id := range cfg.AllowedUserIDs {
			ids[i] = strconv.FormatInt(id, 10)
		}
		if len(ids) > 0 {
			allowedLabel = strings.Join(ids, ", ")
		} else {
			allowedLabel = "none"
		}
	}
	lastActivity := "never"
	mu.Lock()
	if !lastAPISuccessAt.IsZero() {
		lastActivity = formatTimeAgo(lastAPISuccessAt)
	}
	mu.Unlock()

	return fmt.Sprintf(`Telegram relay
Status: %s (%s)
Bot: %s
Chat: %s
Allowed users: %s
Last activity: %s

Commands:
/telegram connect — guided setup
/telegram status — raw deterministic state report
/telegram test — verify outbound and inbound relay
/telegram toggle — enable or disable the relay
/telegram logout — remove saved credentials
/telegram clear — clear Telegram footer and working messages from the TUI`, conn, enabled, botLabel, chatLabel, allowedLabel, lastActivity)
}

func formatTimeAgo(t time.Time) string {
	seconds := int(time.Since(t).Seconds())
	if seconds < 5 {
		return "just now"
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds ago", seconds)
	}
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%dm ago", minutes)
	}
	hours := minutes / 60
	return fmt.Sprintf("%dh ago", hours)
}

// ──────────────────────────────────────────────
// Connect flow
// ──────────────────────────────────────────────

func runConnectFlow(ctx ext.Context) {
	mu.Lock()
	resumePrevious := config != nil && config.Enabled
	mu.Unlock()
	saved := false
	stopPolling()

	defer func() {
		setWorkingMessage("")
		if !saved && resumePrevious {
			ensureDesiredConnection()
		}
		refreshFooter()
	}()

	// 1. Bot token
	tokenResult := ctx.PromptInput(ext.PromptInputConfig{
		Message:     "Bot token from @BotFather",
		Placeholder: "123456789:ABCdef...",
	})
	if tokenResult.Cancelled {
		return
	}
	token := strings.TrimSpace(tokenResult.Value)
	if token == "" {
		ctx.PrintError("Bot token cannot be empty.")
		return
	}

	// 2. Validate token
	setWorkingMessage("Validating bot token...")
	me, err := tgGetMe(token)
	if err != nil {
		setWorkingMessage("")
		ctx.PrintError("Could not validate bot token from @BotFather: " + err.Error())
		return
	}
	setWorkingMessage(fmt.Sprintf("Connected to @%s — preparing chat setup...", me.Username))

	// Capture current offset
	startOffset := captureSetupOffset(token)

	setWorkingMessage("")

	// 3. Chat selection
	resolved := resolveChatTarget(ctx, token, me.ID, startOffset)
	if resolved == nil {
		return
	}

	// 4. Confirm and enable
	chatLabel := fmt.Sprintf("%d", resolved.chatID)
	if resolved.chat.Type != "" {
		chatLabel = fmt.Sprintf("%d (%s)", resolved.chatID, resolved.chat.Type)
	}
	idsStr := make([]string, len(resolved.allowedUserIDs))
	for i, id := range resolved.allowedUserIDs {
		idsStr[i] = strconv.FormatInt(id, 10)
	}

	confirmResult := ctx.PromptConfirm(ext.PromptConfirmConfig{
		Message: fmt.Sprintf("Enable Telegram relay now?\nBot: @%s (%d)\nChat: %s\nAllowed users: %s",
			me.Username, me.ID, chatLabel, strings.Join(idsStr, ", ")),
		DefaultValue: true,
	})
	enableNow := !confirmResult.Cancelled && confirmResult.Value

	newConfig := &RelayConfig{
		Version:         1,
		Enabled:         enableNow,
		BotToken:        token,
		BotID:           me.ID,
		BotUsername:     me.Username,
		ChatID:          resolved.chatID,
		AllowedUserIDs:  resolved.allowedUserIDs,
		LastValidatedAt: time.Now().Format(time.RFC3339),
	}

	stopPolling()
	mu.Lock()
	config = newConfig
	currentOffset = resolved.nextOffset
	offsetInitialized = resolved.hasOffset
	mu.Unlock()

	if err := writeRelayConfig(newConfig); err != nil {
		ctx.PrintError("Failed to save config: " + err.Error())
		return
	}
	recordAPISuccess()
	saved = true
	ensureDesiredConnection()
	refreshFooter()
	ctx.PrintInfo(fmt.Sprintf("Telegram relay configured for @%s, chat %d.", me.Username, resolved.chatID))
}

type resolvedChat struct {
	chat           TelegramChat
	chatID         int64
	allowedUserIDs []int64
	nextOffset     int64
	hasOffset      bool
}

func captureSetupOffset(token string) int64 {
	mu.Lock()
	if offsetInitialized {
		off := currentOffset
		mu.Unlock()
		return off
	}
	mu.Unlock()

	for attempt := 0; attempt < 3; attempt++ {
		updates, err := tgGetUpdates(token, 0, false, 0, 15)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		if len(updates) == 0 {
			return 0
		}
		return updates[len(updates)-1].UpdateID + 1
	}
	return 0
}

func resolveChatTarget(ctx ext.Context, token string, botID int64, startOffset int64) *resolvedChat {
	autoChoice := "Auto-detect after you send /start or a short message to the bot"
	manualChoice := "Enter chat id manually"
	retryChoice := "Retry auto-detect"
	cancelChoice := "Cancel setup"

	selectResult := ctx.PromptSelect(ext.PromptSelectConfig{
		Message: "How should kit find the Telegram chat?",
		Options: []string{autoChoice, manualChoice},
	})
	if selectResult.Cancelled {
		return nil
	}
	mode := selectResult.Value
	nextOffset := startOffset
	hasOffset := startOffset > 0

	for {
		if mode == autoChoice || mode == retryChoice {
			captured, newOffset, newHasOff := captureChatFromMessage(ctx, token, botID, nextOffset, hasOffset)
			nextOffset = newOffset
			hasOffset = newHasOff
			if captured != nil {
				allowedIDs := collectAllowedUserIDs(ctx, captured.chat, captured.senderID)
				if allowedIDs == nil {
					return nil
				}
				return &resolvedChat{
					chat:           captured.chat,
					chatID:         captured.chatID,
					allowedUserIDs: allowedIDs,
					nextOffset:     captured.nextOffset,
					hasOffset:      captured.hasOffset,
				}
			}
			failResult := ctx.PromptSelect(ext.PromptSelectConfig{
				Message: "Telegram chat discovery did not complete.",
				Options: []string{retryChoice, manualChoice, cancelChoice},
			})
			if failResult.Cancelled || failResult.Value == cancelChoice {
				return nil
			}
			mode = failResult.Value
			continue
		}
		if mode == manualChoice {
			return captureChatManually(ctx, token, botID, nextOffset, hasOffset)
		}
		return nil
	}
}

type capturedChat struct {
	chat       TelegramChat
	chatID     int64
	senderID   int64
	nextOffset int64
	hasOffset  bool
}

func captureChatFromMessage(ctx ext.Context, token string, botID int64, startOffset int64, hasOffset bool) (*capturedChat, int64, bool) {
	nextOffset := startOffset
	nextHasOffset := hasOffset
	deadline := time.Now().Add(time.Duration(chatDiscoveryTimeoutMS) * time.Millisecond)
	setWorkingMessage("Waiting for a Telegram message from the selected chat...")

	defer setWorkingMessage("")

	for time.Now().Before(deadline) {
		remainingMs := time.Until(deadline).Milliseconds()
		if remainingMs < 1000 {
			remainingMs = 1000
		}
		timeoutS := int(remainingMs / 1000)
		if timeoutS > chatDiscoveryPollS {
			timeoutS = chatDiscoveryPollS
		}
		if timeoutS < 1 {
			timeoutS = 1
		}
		clientTimeoutS := timeoutS + (clientTimeoutBufferMS / 1000)

		updates, err := tgGetUpdates(token, nextOffset, nextHasOffset, timeoutS, clientTimeoutS)
		if err != nil {
			report("connect.capture.error", err.Error())
			time.Sleep(1 * time.Second)
			continue
		}

		for _, update := range updates {
			nextOffset = update.UpdateID + 1
			nextHasOffset = true
			msg := update.Message
			if msg == nil || msg.Chat.ID == 0 || msg.From.ID == 0 || msg.From.IsBot {
				continue
			}
			setWorkingMessage("Message received — validating chat...")
			chat, err := validateChatForRelay(token, msg.Chat.ID, botID)
			if err != nil {
				report("connect.capture.validate_failure", err.Error())
				continue
			}
			return &capturedChat{
				chat:       *chat,
				chatID:     chat.ID,
				senderID:   msg.From.ID,
				nextOffset: nextOffset,
				hasOffset:  true,
			}, nextOffset, true
		}
	}
	return nil, nextOffset, nextHasOffset
}

func captureChatManually(ctx ext.Context, token string, botID int64, nextOffset int64, hasOffset bool) *resolvedChat {
	inputResult := ctx.PromptInput(ext.PromptInputConfig{
		Message:     "Chat id (message @userinfobot to find it)",
		Placeholder: "-1001234567890",
	})
	if inputResult.Cancelled {
		return nil
	}
	chatID, err := strconv.ParseInt(strings.TrimSpace(inputResult.Value), 10, 64)
	if err != nil {
		ctx.PrintError("Chat id must be numeric.")
		return nil
	}

	setWorkingMessage("Validating chat...")
	chat, err := validateChatForRelay(token, chatID, botID)
	setWorkingMessage("")
	if err != nil {
		ctx.PrintError("Could not validate chat id: " + err.Error())
		return nil
	}

	allowedIDs := collectAllowedUserIDs(ctx, *chat, 0)
	if allowedIDs == nil {
		return nil
	}
	return &resolvedChat{
		chat:           *chat,
		chatID:         chat.ID,
		allowedUserIDs: allowedIDs,
		nextOffset:     nextOffset,
		hasOffset:      hasOffset,
	}
}

func validateChatForRelay(token string, chatID int64, botID int64) (*TelegramChat, error) {
	chat, err := tgGetChat(token, chatID)
	if err != nil {
		return nil, err
	}
	_, err = tgGetChatMember(token, chatID, botID)
	if err != nil {
		return nil, fmt.Errorf("bot is not a member of this chat: %s", err.Error())
	}
	return chat, nil
}

func collectAllowedUserIDs(ctx ext.Context, chat TelegramChat, detectedSenderID int64) []int64 {
	if chat.Type == "private" {
		if detectedSenderID != 0 {
			return []int64{detectedSenderID}
		}
		return []int64{chat.ID}
	}
	// Group/supergroup: ask for CSV
	defaultCSV := "123456789,987654321"
	prompt := "Allowed user ids, comma-separated (message @userinfobot to find them)"
	if detectedSenderID != 0 {
		defaultCSV = strconv.FormatInt(detectedSenderID, 10)
		prompt = fmt.Sprintf("Allowed user ids, comma-separated (detected sender: %d; others can message @userinfobot)", detectedSenderID)
	}

	inputResult := ctx.PromptInput(ext.PromptInputConfig{
		Message:     prompt,
		Placeholder: defaultCSV,
	})
	if inputResult.Cancelled {
		return nil
	}
	ids := parseAllowedUserIDs(inputResult.Value)
	if len(ids) == 0 {
		ctx.PrintError("Allowed user ids must contain at least one numeric id.")
		return nil
	}
	return ids
}

func parseAllowedUserIDs(csv string) []int64 {
	parts := strings.Split(csv, ",")
	seen := make(map[int64]bool)
	var ids []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			continue
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// ──────────────────────────────────────────────
// Startup / shutdown messages
// ──────────────────────────────────────────────

func sendStartupConnectedMessage() {
	mu.Lock()
	cfg := config
	mu.Unlock()
	if cfg == nil || !cfg.Enabled || cfg.LastValidatedAt == "" {
		return
	}
	telegramSend(fmt.Sprintf("🟢 %s connected", botRef()))
}

func sendShutdownDisconnectedMessage() {
	if !isConnected() {
		return
	}
	telegramSend(fmt.Sprintf("🔴 %s disconnected", botRef()))
}

// ──────────────────────────────────────────────
// Init — wire everything into Kit
// ──────────────────────────────────────────────

func Init(api ext.API) {
	// Check for debug mode
	if os.Getenv("KIT_TELEGRAM_DEBUG") == "1" {
		debugMode = true
	}

	// Initialize queue
	initQueue()
	nextRunID = 1

	// Register the /telegram command
	api.RegisterCommand(ext.CommandDef{
		Name:        "telegram",
		Description: "Manage the Telegram relay",
		Execute: func(args string, ctx ext.Context) (string, error) {
			mu.Lock()
			latestCtx = ctx
			latestCtxSet = true
			mu.Unlock()

			command := strings.TrimSpace(args)
			parts := strings.Fields(command)
			sub := ""
			if len(parts) > 0 {
				sub = parts[0]
			}

			switch sub {
			case "":
				ctx.PrintInfo(buildCommandHelpText())
			case "status":
				ctx.PrintInfo(buildStatusText())
			case "connect":
				runConnectFlow(ctx)
			case "toggle":
				toggleRelay(ctx)
			case "logout":
				logoutRelay(ctx)
			case "test":
				runRelayTest(ctx)
			case "clear":
				setWorkingMessage("")
				clearFooter()
				ctx.PrintInfo("Telegram TUI cleared.")
			default:
				ctx.PrintInfo(buildCommandHelpText())
			}
			return "", nil
		},
		Complete: func(prefix string, ctx ext.Context) []string {
			options := []string{"connect", "status", "test", "toggle", "logout", "clear"}
			if prefix == "" {
				return options
			}
			var matches []string
			for _, opt := range options {
				if strings.HasPrefix(opt, prefix) {
					matches = append(matches, opt)
				}
			}
			return matches
		},
	})

	// Session start: load config, start polling
	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		latestCtxSet = true
		projectDir = ctx.CWD
		mu.Unlock()

		cfg, err := readRelayConfig()
		if err != nil {
			report("session_start.config_error", err.Error())
		}
		mu.Lock()
		config = cfg
		mu.Unlock()

		ensureHealthTimer()
		sendStartupConnectedMessage()
		ensureDesiredConnection()
		refreshFooter()
		report("session_start", fmt.Sprintf("configPresent=%v enabled=%v", cfg != nil, cfg != nil && cfg.Enabled))
	})

	// Session shutdown: disconnect
	api.OnSessionShutdown(func(e ext.SessionShutdownEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		latestCtxSet = true
		mu.Unlock()

		sendShutdownDisconnectedMessage()
		stopPolling()
		clearHealthTimer()
		clearFooter()
	})

	// Agent start: new run
	api.OnAgentStart(func(e ext.AgentStartEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		latestCtxSet = true
		run := &ActiveRunState{
			ID:        nextRunID,
			StartedAt: time.Now(),
			StepCount: 1,
			Actions:   make([]RenderAction, 0),
		}
		nextRunID++
		activeRun = run
		agentBusy = true
		lastProgressEditAt = time.Time{}
		mu.Unlock()

		report("run.start", fmt.Sprintf("runId=%d", run.ID))
		ensureProgressMessage()
		updateProgressMessage()
	})

	// Agent end: finalize run, drain queue
	api.OnAgentEnd(func(e ext.AgentEndEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		latestCtxSet = true
		run := activeRun
		mu.Unlock()

		if run != nil {
			// Capture final response from event
			if e.Response != "" {
				mu.Lock()
				run.LastAssistantText = e.Response
				run.LastAssistantError = (e.StopReason == "error")
				mu.Unlock()
			}
			finalizeRun()
		}

		mu.Lock()
		activeRun = nil
		agentBusy = false
		mu.Unlock()

		// Drain queue: promote next item to new run
		promoteOneToNewRun()
	})

	// Tool call: track action start
	api.OnToolCall(func(e ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		mu.Lock()
		latestCtx = ctx
		latestCtxSet = true
		run := activeRun
		if run != nil {
			run.Actions = append(run.Actions, RenderAction{
				ID:     e.ToolCallID,
				Label:  summarizeToolAction(e.ToolName, e.Input),
				Status: "running",
			})
		}
		mu.Unlock()

		go updateProgressMessage()
		return nil
	})

	// Tool result: track action completion
	api.OnToolResult(func(e ext.ToolResultEvent, ctx ext.Context) *ext.ToolResultResult {
		mu.Lock()
		latestCtx = ctx
		latestCtxSet = true
		run := activeRun
		if run != nil {
			// Find and update existing action, or add new one
			found := false
			for i := range run.Actions {
				if run.Actions[i].Status == "running" && run.Actions[i].Label == summarizeToolAction(e.ToolName, e.Input) {
					if e.IsError {
						run.Actions[i].Status = "error"
					} else {
						run.Actions[i].Status = "done"
					}
					run.Actions[i].Label = summarizeToolResult(e.ToolName, e.Input, e.IsError)
					found = true
					break
				}
			}
			if !found {
				status := "done"
				if e.IsError {
					status = "error"
				}
				run.Actions = append(run.Actions, RenderAction{
					ID:     e.ToolName,
					Label:  summarizeToolResult(e.ToolName, e.Input, e.IsError),
					Status: status,
				})
			}
			run.StepCount++
		}
		mu.Unlock()

		go updateProgressMessage()
		return nil
	})

	// Message end: capture assistant text
	api.OnMessageEnd(func(e ext.MessageEndEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		latestCtxSet = true
		run := activeRun
		if run != nil && e.Content != "" {
			run.LastAssistantText = e.Content
		}
		mu.Unlock()
	})

	// Input: mirror local user messages to Telegram
	api.OnInput(func(e ext.InputEvent, ctx ext.Context) *ext.InputResult {
		mu.Lock()
		latestCtx = ctx
		latestCtxSet = true
		mu.Unlock()
		report("input", fmt.Sprintf("source=%s text=%s", e.Source, truncate(e.Text, 60)))

		// Mirror locally-typed messages to Telegram so the remote side sees the full conversation.
		// Skip messages that originated from Telegram (injected via SendMessage as "queue" source)
		// and skip empty or command inputs.
		if e.Source == "interactive" || e.Source == "cli" || e.Source == "script" {
			text := strings.TrimSpace(e.Text)
			if text != "" && !strings.HasPrefix(text, "/") {
				go func() {
					if isConnected() {
						telegramSend("💬 " + text)
					}
				}()
			}
		}
		return nil
	})

	// Register option for debug mode
	api.RegisterOption(ext.OptionDef{
		Name:        "telegram-debug",
		Description: "Enable debug logging for kit-telegram (set KIT_TELEGRAM_DEBUG=1)",
		Default:     "false",
	})
}
