package artifacts

import "testing"

func TestBoundedJSONUsesArtifactsResultEnvelope(t *testing.T) {
	var commits []Commit
	if err := decodeBoundedResult([]byte(`{"success":true,"result":[{"hash":"commit","treeHash":"tree","author":{"name":"Alice"}}],"errors":[]}`), &commits); err != nil || len(commits) != 1 || commits[0].Hash != "commit" {
		t.Fatalf("enveloped commit not decoded: %+v %v", commits, err)
	}
	for _, body := range []string{`{"errors":[{"message":"private provider detail"}],"result":[]}`, `[]`, `{"result":null}`} {
		var entries []TreeEntry
		if err := decodeBoundedResult([]byte(body), &entries); err == nil {
			t.Fatal("missing/error envelope accepted")
		}
	}
}
