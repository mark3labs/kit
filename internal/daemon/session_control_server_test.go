package daemon

import (
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestSessionControlRenameResult(t *testing.T) {
	table := newTestTable(t)
	sess := table.fakeSession(11)
	var buf lockedBuffer
	conn := table.conns.addLocal(newFrameSink(&buf))

	payload := make([]byte, 8+len("  renamed  "))
	binary.BigEndian.PutUint64(payload, sess.id)
	copy(payload[8:], "  renamed  ")
	if err := table.renameSession(payload); err != nil {
		t.Fatalf("renameSession: %v", err)
	}
	if got := sess.displayName(); got != "renamed" {
		t.Fatalf("name = %q, want renamed", got)
	}
	table.sendSessionControlResult(conn.id, nil)
	assertControlResult(t, &buf, "")
}

func TestSessionControlErrorsForMissingAndMalformedRequests(t *testing.T) {
	table := newTestTable(t)
	if err := table.renameSession([]byte{1}); err == nil {
		t.Fatal("malformed rename must fail")
	}
	if err := table.renameSession(encodeSessionID(99)); err == nil {
		t.Fatal("rename of missing session must fail")
	}
	if err := table.killSession([]byte{1}); err == nil {
		t.Fatal("malformed kill must fail")
	}
	if err := table.killSession(encodeSessionID(99)); err == nil {
		t.Fatal("kill of missing session must fail")
	}
}

func TestKillSessionRetiresAndReports(t *testing.T) {
	table := newTestTable(t)
	sess := table.fakeSession(12)
	var buf lockedBuffer
	conn := table.conns.addLocal(newFrameSink(&buf))
	table.mu.Lock()
	table.wireMap[conn.id] = sess.id
	table.mu.Unlock()
	sess.attachClient(conn.id, winSize{})

	if err := table.killSession(encodeSessionID(sess.id)); err != nil {
		t.Fatalf("killSession: %v", err)
	}
	if table.sessionCount() != 0 {
		t.Fatal("kill must remove session from table")
	}
	frame, err := ReadFrame(&buf)
	if err != nil || frame.Type != FrameBye {
		t.Fatalf("kill notification = (%+v, %v), want BYE", frame, err)
	}
	if err := table.killSession(encodeSessionID(sess.id)); err == nil {
		t.Fatal("second kill must report missing session")
	}
}

func assertControlResult(t *testing.T, buf *lockedBuffer, want string) {
	t.Helper()
	frame, err := ReadFrame(buf)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	if frame.Type != FrameSessionControlResult {
		t.Fatalf("frame type = %#x, want control result", frame.Type)
	}
	var result struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(frame.Payload, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Error != want {
		t.Fatalf("result error = %q, want %q", result.Error, want)
	}
}
