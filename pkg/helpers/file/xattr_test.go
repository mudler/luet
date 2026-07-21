package file

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLgetxattrReturnsValue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []byte("hello")
	if err := unix.Lsetxattr(p, "user.test", want, 0); err != nil {
		t.Skipf("xattrs unsupported on this filesystem: %v", err)
	}

	got, err := lgetxattr(p, "user.test")
	if err != nil {
		t.Fatalf("lgetxattr: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Absent attributes must come back as (nil, nil), not an error.
// copyXattr depends on this to skip attributes that are not set.
func TestLgetxattrAbsentReturnsNil(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := lgetxattr(p, "user.does.not.exist")
	if err != nil {
		t.Fatalf("expected nil error for absent attr, got %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil value for absent attr, got %q", got)
	}
}

// Values larger than the initial 128-byte buffer exercise the ERANGE
// resize path.
func TestLgetxattrLargeValue(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := []byte(strings.Repeat("a", 1024))
	if err := unix.Lsetxattr(p, "user.big", want, 0); err != nil {
		t.Skipf("xattrs unsupported on this filesystem: %v", err)
	}

	got, err := lgetxattr(p, "user.big")
	if err != nil {
		t.Fatalf("lgetxattr: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %d bytes, want %d", len(got), len(want))
	}
}
