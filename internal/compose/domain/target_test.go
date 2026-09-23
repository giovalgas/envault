package domain

import (
	"encoding/json"
	"testing"
)

func TestGitignoreStatus(t *testing.T) {
	cases := []struct {
		status GitignoreStatus
		known  bool
		json   string
		text   string
	}{
		{GitignoreIgnored, true, "true", "ignored"},
		{GitignoreNotIgnored, true, "false", "not-ignored"},
		{GitignoreUnknown, false, "null", "unknown"},
	}
	for _, tc := range cases {
		data, err := json.Marshal(struct {
			Gitignored GitignoreStatus `json:"gitignored"`
		}{tc.status})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(data) != `{"gitignored":`+tc.json+`}` {
			t.Errorf("%v: json = %s", tc.status, data)
		}
		if tc.status.Known() != tc.known || tc.status.String() != tc.text {
			t.Errorf("%v: Known = %v String = %q", tc.status, tc.status.Known(), tc.status.String())
		}
	}
}
