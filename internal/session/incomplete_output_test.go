package session

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"charm.land/fantasy"
)

func TestIncompleteOutputSurvivesReloadOutsideModelHistory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := tm.GetFilePath()
	if _, err := tm.AppendStep(context.Background(), []fantasy.Message{fantasy.NewUserMessage("go")}); err != nil {
		t.Fatal(err)
	}
	id, err := tm.AppendIncompleteOutput(context.Background(), "attempt-1", "unfinished")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tm.AppendStep(context.Background(), []fantasy.Message{fantasy.NewUserMessage("continue")}); err != nil {
		t.Fatal(err)
	}
	if err := tm.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"type":"incomplete_output"`) || !strings.Contains(string(data), `"incomplete":true`) {
		t.Fatalf("JSONL=%s", data)
	}
	reopened, err := OpenTreeSession(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Error(err)
		}
	}()
	entry, ok := reopened.GetEntry(id).(*IncompleteOutputEntry)
	if !ok || entry.AttemptID != "attempt-1" || entry.Text != "unfinished" || !entry.Incomplete {
		t.Fatalf("entry=%#v", entry)
	}
	if got := len(reopened.GetLLMMessages()); got != 2 {
		t.Fatalf("history length=%d", got)
	}
	ctx, _, _ := reopened.BuildContext()
	if len(ctx) != 2 {
		t.Fatalf("model context=%v", ctx)
	}
	if len(reopened.GetBranch("")) != 3 {
		t.Fatal("incomplete entry lost from tree")
	}
}

func TestIncompleteOutputSyncFailureDoesNotAdvanceTree(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tm, err := CreateTreeSession(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := tm.Close(); err != nil {
			t.Error(err)
		}
	}()
	failure := errors.New("sync failed")
	tm.syncHook = func() error { return failure }
	id, err := tm.AppendIncompleteOutput(context.Background(), "attempt", "unfinished")
	if id != "" || !errors.Is(err, failure) {
		t.Fatalf("id=%q error=%v", id, err)
	}
	if len(tm.GetBranch("")) != 0 || countLines(t, tm.GetFilePath()) != 1 {
		t.Fatal("failed write changed tree or file")
	}
	tm.syncHook = nil
	if _, err := tm.AppendIncompleteOutput(context.Background(), "attempt", "unfinished"); err != nil {
		t.Fatal(err)
	}
}
