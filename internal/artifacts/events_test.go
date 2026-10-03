package artifacts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeDocumentedEventsAndSafeIdentity(t *testing.T) {
	for _, kind := range []string{"created", "imported", "forked", "deleted", "pushed", "cloned", "fetched", "token.created", "token.revoked"} {
		raw := map[string]any{"type": "cf.artifacts.repo." + kind, "source": map[string]any{"namespace": "ns", "repoName": "repo"}, "metadata": map[string]any{"eventTimestamp": "2026-10-03T00:00:00Z", "accountId": "account", "eventSubscriptionId": "one"}, "payload": map[string]any{"ref": "refs/heads/main", "before": strings.Repeat("a", 40), "after": strings.Repeat("b", 40), "plaintext": "SECRET", "sourceUrl": "https://user:SECRET@example.com", "tokenId": "token-id", "scope": "read"}}
		data, _ := json.Marshal(raw)
		first, err := NormalizeEvent(data, "ns")
		if err != nil {
			t.Fatal(kind, err)
		}
		raw["metadata"].(map[string]any)["eventSubscriptionId"] = "another"
		data, _ = json.Marshal(raw)
		second, err := NormalizeEvent(data, "ns")
		if err != nil || first.ID != second.ID {
			t.Fatal("delivery-dependent identity")
		}
		encoded, _ := json.Marshal(first)
		if strings.Contains(string(encoded), "SECRET") {
			t.Fatal("secret persisted")
		}
		if _, err := NormalizeEvent(data, "other"); err == nil {
			t.Fatal("cross namespace accepted")
		}
	}
}
