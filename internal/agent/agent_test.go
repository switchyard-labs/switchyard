package agent

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestMalformedCiphertextDoesNotPanic(t *testing.T) {
	c := NewCredentialStore(nil, nil, nil, make([]byte, 32))
	for _, raw := range []string{"", base64.StdEncoding.EncodeToString([]byte{1}), "not-base64"} {
		if _, err := c.decrypt(raw); err == nil {
			t.Fatal("accepted malformed ciphertext")
		}
	}
}
func TestRotationRetainsOriginalOnPersistenceFailure(t *testing.T) {
	deleted := false
	old := "old ciphertext"
	record := map[string]any{"id": "cred", "ciphertext": old}
	c := NewCredentialStore(nil, func() ([]map[string]any, error) { return []map[string]any{record}, nil }, func(string) error { deleted = true; return nil }, make([]byte, 32))
	c.SetUpdater(func(string, string) error { return errors.New("injected persistence failure") })
	if c.Rotate("cred", "new secret") == nil {
		t.Fatal("returned success")
	}
	if deleted || record["ciphertext"] != old {
		t.Fatal("lost original")
	}
}
func TestOutputLimit(t *testing.T) {
	var b limitedOutput
	s := strings.Repeat("x", 2<<20)
	n, err := b.Write([]byte(s))
	if n != len(s) || err != nil || b.Len() != 1<<20 {
		t.Fatal(n, err, b.Len())
	}
}
