package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeyFailureDoesNotReplaceKey(t *testing.T) {
	t.Setenv("SWITCHYARD_SECRET_KEY", "")
	os.Unsetenv("SWITCHYARD_SECRET_KEY")
	p := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(p, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateKey(p); err == nil {
		t.Fatal("bad key accepted")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "broken" {
		t.Fatal("key replaced")
	}
}
func TestKeyCreatedPrivateAndStable(t *testing.T) {
	t.Setenv("SWITCHYARD_SECRET_KEY", "")
	os.Unsetenv("SWITCHYARD_SECRET_KEY")
	p := filepath.Join(t.TempDir(), "key")
	a, err := loadOrCreateKey(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := loadOrCreateKey(p)
	if err != nil || string(a) != string(b) {
		t.Fatal("key unstable", err)
	}
	info, _ := os.Stat(p)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
}
