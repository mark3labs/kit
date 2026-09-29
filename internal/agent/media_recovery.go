package agent

import (
	"errors"
	"fmt"
	"net/http"

	"charm.land/fantasy"
)

// This file implements recovery from a provider that rejects a media tool
// result (for example, an image from the read tool sent to a model without
// vision support, or an image the provider cannot process).
//
// A tool can return an image without error, but the provider only sees it on
// the NEXT request. If the provider rejects that request, the LLM library
// aborts the whole turn. Worse, the step that holds the image was already
// persisted, so every later turn replays the image and fails the same way.
//
// To prevent this, the agent holds back persistence of a step that contains
// media tool results until the provider accepts the request that carries it.
// If the provider rejects it with a client error (400, 413 request too
// large, context overflow), the media results are
// replaced with error results that contain the provider message, the fixed
// messages are persisted, and the turn continues. The model then sees
// why the image failed and can recover.

// mediaRejectedPrefix starts the error text that replaces a rejected media
// tool result.
const mediaRejectedPrefix = "the tool result could not be sent to the model"

// hasMediaToolResult reports whether any message contains a tool result whose
// output is media.
func hasMediaToolResult(msgs []fantasy.Message) bool {
	for _, msg := range msgs {
		for _, part := range msg.Content {
			trp, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part)
			if !ok {
				continue
			}
			if _, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](trp.Output); ok {
				return true
			}
		}
	}
	return false
}

// maxMediaRetries bounds how many times one turn is replayed after a
// provider rejected a media tool result. Each replay removes the media that
// caused it, so the limit only stops a model that reads a new rejected image
// on every step.
const maxMediaRetries = 3

// isMediaRejection reports whether err is a provider error that a rejected
// media payload can cause: a client error such as 400 (image not supported
// or not valid), 413 (request above the provider size limit, for example
// Anthropic "request_too_large"), or a context overflow. Authentication,
// rate limit, timeout and server errors are not about the request content,
// and dropping the media for them would lose data for no reason.
func isMediaRejection(err error) bool {
	var pe *fantasy.ProviderError
	if !errors.As(err, &pe) {
		return false
	}
	if pe.IsContextTooLarge() {
		return true
	}
	if pe.AuthError || pe.TransientError || pe.IsRetryable() {
		return false
	}
	switch pe.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return false
	}
	return pe.StatusCode >= 400 && pe.StatusCode < 500
}

// replaceMediaToolResults returns a copy of msgs in which every media tool
// result is replaced by an error result that tells the model why the media
// was not delivered. Messages without media results are shared, not copied.
// The second return value is the number of replaced results.
func replaceMediaToolResults(msgs []fantasy.Message, cause error) ([]fantasy.Message, int) {
	out := make([]fantasy.Message, len(msgs))
	replaced := 0
	for i, msg := range msgs {
		out[i] = msg
		if !hasMediaToolResult([]fantasy.Message{msg}) {
			continue
		}
		content := make([]fantasy.MessagePart, len(msg.Content))
		for j, part := range msg.Content {
			content[j] = part
			trp, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part)
			if !ok {
				continue
			}
			media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](trp.Output)
			if !ok {
				continue
			}
			trp.Output = fantasy.ToolResultOutputContentError{
				Error: errors.New(mediaRejectedText(media, cause)),
			}
			content[j] = trp
			replaced++
		}
		out[i].Content = content
	}
	return out, replaced
}

// mediaRejectedText builds the error text for a rejected media result. It
// keeps the tool summary (for example the file path) so the model knows which
// result failed.
func mediaRejectedText(media fantasy.ToolResultOutputContentMedia, cause error) string {
	what := media.Text
	if what == "" {
		what = media.MediaType + " content"
	}
	return fmt.Sprintf("%s: %s. The provider rejected the request that carried it: %v. %s",
		mediaRejectedPrefix, what, cause, mediaRejectedAdvice(cause))
}

// mediaRejectedAdvice tells the model what to do next, based on why the
// provider rejected the request.
func mediaRejectedAdvice(cause error) string {
	if isSizeRejection(cause) {
		return "The request was too large: the conversation already holds a lot of media. " +
			"Do not read the same files again. Read fewer images at a time, " +
			"or continue with what you already know."
	}
	return "The current model may not accept this media. " +
		"Do not retry the same call; use another approach."
}

// isSizeRejection reports whether err is a provider error for a request
// above the provider size limit (HTTP 413) or the context window.
func isSizeRejection(err error) bool {
	var pe *fantasy.ProviderError
	if !errors.As(err, &pe) {
		return false
	}
	return pe.StatusCode == http.StatusRequestEntityTooLarge || pe.IsContextTooLarge()
}
