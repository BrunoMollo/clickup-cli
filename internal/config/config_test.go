package config

import (
	"bytes"
	"strings"
	"testing"
)

func TestParsePrecedenceAndDefaults(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		env        map[string]string
		wantAnchor string
		wantClosed bool
	}{
		{name: "default", env: map[string]string{"CLICKUP_API_TOKEN": "secret"}, wantAnchor: DefaultAnchorView},
		{name: "environment", env: map[string]string{"CLICKUP_API_TOKEN": "secret", "CLICKUP_ANCHOR_VIEW": "env-view"}, wantAnchor: "env-view"},
		{name: "flag", args: []string{"--anchor-view", "flag-view", "--include-closed"}, env: map[string]string{"CLICKUP_API_TOKEN": "secret", "CLICKUP_ANCHOR_VIEW": "env-view"}, wantAnchor: "flag-view", wantClosed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := Parse(test.args, func(key string) string { return test.env[key] }, &bytes.Buffer{})
			if err != nil {
				t.Fatal(err)
			}
			if cfg.AnchorView != test.wantAnchor || cfg.IncludeClosed != test.wantClosed {
				t.Fatalf("config inesperada: %+v", cfg)
			}
		})
	}
}

func TestParseRequiresToken(t *testing.T) {
	_, err := Parse(nil, func(string) string { return "" }, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "CLICKUP_API_TOKEN") {
		t.Fatalf("error inesperado: %v", err)
	}
}
