package app

import (
	"reflect"
	"strings"
	"testing"
)

func TestReviewContractPathsIncludesUnchangedCounterpartsOnce(t *testing.T) {
	paths, err := reviewContractPaths([]byte(`{"rules":[{"kind":"field_equals","a":"api/version.json:version","b":"consumer.json:requires"},{"kind":"field_equals","a":"api/version.json:version","b":"api/version.json:requires"}]}`))
	if err != nil || !reflect.DeepEqual(paths, []string{"switchyard.contract.json", "api/version.json", "consumer.json"}) {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
}

func TestReviewContractPathsRejectsUnsafeAndUnboundedInputs(t *testing.T) {
	for _, selector := range []string{"../secret.json:v", "/secret.json:v", "a/../secret.json:v", `a\\b.json:v`, "file.json", "file.json:"} {
		contract := `{"rules":[{"a":"` + strings.ReplaceAll(selector, `\`, `\\`) + `","b":"b.json:v"}]}`
		if _, err := reviewContractPaths([]byte(contract)); err == nil {
			t.Fatalf("accepted %q", selector)
		}
	}
	for _, input := range []string{`{`, strings.Repeat(" ", (64<<10)+1), `{"rules":[` + strings.Repeat(`{"a":"a:v","b":"b:v"},`, 100) + `{"a":"a:v","b":"b:v"}]}`} {
		if _, err := reviewContractPaths([]byte(input)); err == nil {
			t.Fatal("accepted invalid/oversized contract")
		}
	}
}
