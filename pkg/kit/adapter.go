package kit

import (
	"strings"
	"time"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/session"
)

// treeManagerAdapter adapts TreeManager to SessionManager interface.
// This is unexported - users don't interact with it directly.
type treeManagerAdapter struct {
	inner *session.TreeManager
}

// NewTreeManagerAdapter creates an adapter (exported for use in New function).
// This is used by the SDK when no custom SessionManager is provided.
func NewTreeManagerAdapter(tm *session.TreeManager) SessionManager {
	return &treeManagerAdapter{inner: tm}
}

// AppendMessage implements SessionManager.
func (a *treeManagerAdapter) AppendMessage(msg LLMMessage) (string, error) {
	// LLMMessage is just an alias for fantasy.Message, so no conversion needed
	return a.inner.AppendLLMMessage(msg)
}

// GetMessages implements SessionManager.
func (a *treeManagerAdapter) GetMessages() []LLMMessage {
	// LLMMessage is just an alias for fantasy.Message
	return a.inner.GetLLMMessages()
}

// BuildContext implements SessionManager.
func (a *treeManagerAdapter) BuildContext() ([]LLMMessage, string, string) {
	msgs, provider, modelID := a.inner.BuildContext()
	return msgs, provider, modelID
}

// Branch implements SessionManager.
func (a *treeManagerAdapter) Branch(entryID string) error {
	return a.inner.Branch(entryID)
}

// GetCurrentBranch implements SessionManager.
func (a *treeManagerAdapter) GetCurrentBranch() []BranchEntry {
	branch := a.inner.GetBranch("")
	var result []BranchEntry
	for _, entry := range branch {
		be := a.convertEntry(entry)
		if be != nil {
			result = append(result, *be)
		}
	}
	return result
}

// GetChildren implements SessionManager.
func (a *treeManagerAdapter) GetChildren(parentID string) []string {
	return a.inner.GetChildren(parentID)
}

// GetEntry implements SessionManager.
func (a *treeManagerAdapter) GetEntry(entryID string) *BranchEntry {
	entry := a.inner.GetEntry(entryID)
	if entry == nil {
		return nil
	}
	return a.convertEntry(entry)
}

// GetSessionID implements SessionManager.
func (a *treeManagerAdapter) GetSessionID() string {
	return a.inner.GetSessionID()
}

// GetSessionName implements SessionManager.
func (a *treeManagerAdapter) GetSessionName() string {
	return a.inner.GetSessionName()
}

// SetSessionName implements SessionManager.
func (a *treeManagerAdapter) SetSessionName(name string) error {
	_, err := a.inner.AppendSessionInfo(name)
	return err
}

// GetCreatedAt implements SessionManager.
func (a *treeManagerAdapter) GetCreatedAt() time.Time {
	return a.inner.GetHeader().Timestamp
}

// IsPersisted implements SessionManager.
func (a *treeManagerAdapter) IsPersisted() bool {
	return a.inner.IsPersisted()
}

// AppendCompaction implements SessionManager.
func (a *treeManagerAdapter) AppendCompaction(summary string, firstKeptEntryID string,
	tokensBefore, tokensAfter int, messagesRemoved int, readFiles, modifiedFiles []string) (string, error) {

	return a.inner.AppendCompaction(summary, firstKeptEntryID,
		tokensBefore, tokensAfter, messagesRemoved, readFiles, modifiedFiles)
}

// GetLastCompaction implements SessionManager.
func (a *treeManagerAdapter) GetLastCompaction() *CompactionEntry {
	c := a.inner.GetLastCompaction()
	if c == nil {
		return nil
	}
	return &CompactionEntry{
		ID:               c.ID,
		Summary:          c.Summary,
		FirstKeptEntryID: c.FirstKeptEntryID,
		TokensBefore:     c.TokensBefore,
		TokensAfter:      c.TokensAfter,
		MessagesRemoved:  c.MessagesRemoved,
		ReadFiles:        c.ReadFiles,
		ModifiedFiles:    c.ModifiedFiles,
		Timestamp:        c.Timestamp,
	}
}

// AppendExtensionData implements SessionManager.
func (a *treeManagerAdapter) AppendExtensionData(extType, data string) (string, error) {
	return a.inner.AppendExtensionData(extType, data)
}

// GetExtensionData implements SessionManager.
func (a *treeManagerAdapter) GetExtensionData(extType string) []ExtensionDataEntry {
	entries := a.inner.GetExtensionData(extType)
	var result []ExtensionDataEntry
	for _, e := range entries {
		result = append(result, ExtensionDataEntry{
			ID:        e.ID,
			ExtType:   e.ExtType,
			Data:      e.Data,
			Timestamp: e.Timestamp,
		})
	}
	return result
}

// AppendModelChange implements SessionManager.
func (a *treeManagerAdapter) AppendModelChange(provider, modelID string) (string, error) {
	return a.inner.AppendModelChange(provider, modelID)
}

// GetContextEntryIDs implements SessionManager.
func (a *treeManagerAdapter) GetContextEntryIDs() []string {
	return a.inner.GetContextEntryIDs()
}

// Close implements SessionManager.
func (a *treeManagerAdapter) Close() error {
	return a.inner.Close()
}

// Helper: Convert internal entry types to BranchEntry
func (a *treeManagerAdapter) convertEntry(entry any) *BranchEntry {
	switch e := entry.(type) {
	case *session.MessageEntry:
		msg, err := e.ToMessage()
		if err != nil {
			return nil
		}
		// Build content text from parts
		var content strings.Builder
		for _, part := range msg.Parts {
			if textPart, ok := part.(TextContent); ok {
				content.WriteString(textPart.Text)
			}
		}
		return &BranchEntry{
			ID:        e.ID,
			ParentID:  e.ParentID,
			Type:      EntryTypeMessage,
			Role:      string(msg.Role),
			Content:   content.String(),
			Model:     e.Model,
			Provider:  e.Provider,
			Timestamp: e.Timestamp,
			RawParts:  msg.Parts,
		}
	case *session.BranchSummaryEntry:
		return &BranchEntry{
			ID:        e.ID,
			ParentID:  e.ParentID,
			Type:      EntryTypeBranchSummary,
			Content:   e.Summary,
			Timestamp: e.Timestamp,
		}
	case *session.ModelChangeEntry:
		return &BranchEntry{
			ID:        e.ID,
			ParentID:  e.ParentID,
			Type:      EntryTypeModelChange,
			Content:   "Model changed to " + e.Provider + "/" + e.ModelID,
			Model:     e.ModelID,
			Provider:  e.Provider,
			Timestamp: e.Timestamp,
		}
	case *session.CompactionEntry:
		return &BranchEntry{
			ID:        e.ID,
			ParentID:  e.ParentID,
			Type:      EntryTypeCompaction,
			Content:   e.Summary,
			Timestamp: e.Timestamp,
		}
	case *session.ExtensionDataEntry:
		return &BranchEntry{
			ID:        e.ID,
			ParentID:  e.ParentID,
			Type:      EntryTypeExtensionData,
			Content:   "Extension data: " + e.ExtType,
			Timestamp: e.Timestamp,
		}
	default:
		return nil
	}
}

// convertKitMessagesToFantasy converts kit LLM messages to fantasy messages.
// Since LLMMessage is an alias for fantasy.Message, this is a no-op.
func convertKitMessagesToFantasy(msgs []LLMMessage) []fantasy.Message {
	// LLMMessage is just an alias for fantasy.Message, so we can type convert
	return msgs
}
