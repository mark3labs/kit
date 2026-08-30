package clipboard

import (
	"bytes"
	"os"
	"testing"
)

func TestReadImageRemoteClipboardEnv(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 13}
	dir := t.TempDir()
	path := dir + "/clip"

	// Remote clipboard file present: ReadImage serves it.
	t.Setenv(RemoteClipboardEnv, path)
	if err := os.WriteFile(path, png, 0o600); err != nil {
		t.Fatal(err)
	}
	img, err := ReadImage()
	if err != nil {
		t.Fatalf("ReadImage: %v", err)
	}
	if !bytes.Equal(img.Data, png) || img.MediaType != "image/png" {
		t.Fatalf("unexpected image: %d bytes %s", len(img.Data), img.MediaType)
	}

	// Empty file (the daemon's "no image" clear): must be a soft error.
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadImage(); err == nil {
		t.Fatal("empty remote clipboard should error")
	}

	// Unrecognized content: soft error, not a bogus attachment.
	if err := os.WriteFile(path, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadImage(); err == nil {
		t.Fatal("unrecognized content should error")
	}
}
