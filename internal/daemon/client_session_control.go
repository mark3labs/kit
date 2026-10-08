package daemon

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// RenameSession changes the display name of a session on the local daemon.
func RenameSession(ctx context.Context, id uint64, name string) error {
	conn, err := DialLocal(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return controlSession(ctx, conn, FrameSessionRename, id, name)
}

// KillSession stops a session on the local daemon.
func KillSession(ctx context.Context, id uint64) error {
	conn, err := DialLocal(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	return controlSession(ctx, conn, FrameSessionKill, id, "")
}

// RenameHostSession changes a session name on a paired daemon.
func RenameHostSession(ctx context.Context, host string, id uint64, name string) error {
	conn, err := dialControlHost(ctx, host)
	if err != nil {
		return err
	}
	defer conn.Close()
	return controlSession(ctx, conn, FrameSessionRename, id, name)
}

// KillHostSession stops a session on a paired daemon.
func KillHostSession(ctx context.Context, host string, id uint64) error {
	conn, err := dialControlHost(ctx, host)
	if err != nil {
		return err
	}
	defer conn.Close()
	return controlSession(ctx, conn, FrameSessionKill, id, "")
}

func dialControlHost(ctx context.Context, host string) (*remoteConn, error) {
	entry, err := GetHost(host)
	if err != nil {
		return nil, err
	}
	return dialHostQuiet(ctx, host, entry, true)
}

func controlSession(ctx context.Context, conn io.ReadWriter, typ FrameType, id uint64, name string) error {
	cc := newClientConn(conn)
	go cc.readLoop()
	peer, err := cc.greet()
	if err != nil {
		return err
	}
	if !peer.Features.Has(FeatureSessionControl) {
		return fmt.Errorf("daemon does not support session management; upgrade and restart it")
	}
	if typ == FrameSessionRename {
		payload := make([]byte, 8+len(name))
		binary.BigEndian.PutUint64(payload, id)
		copy(payload[8:], name)
		if err := cc.write(typ, payload); err != nil {
			return err
		}
	} else {
		payload := make([]byte, 8)
		binary.BigEndian.PutUint64(payload, id)
		if err := cc.write(typ, payload); err != nil {
			return err
		}
	}
	frame, err := cc.awaitCtrl(ctx, FrameSessionControlResult, 10*time.Second)
	if err != nil {
		return err
	}
	var result struct {
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(frame.Payload, &result); err != nil {
		return fmt.Errorf("daemon: bad session control result: %w", err)
	}
	if result.Error != "" {
		return fmt.Errorf("daemon: %s", result.Error)
	}
	return nil
}
