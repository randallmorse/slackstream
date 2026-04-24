package slack

import (
	"testing"
)

func TestNewClient(t *testing.T) {
	client := NewClient("xoxp-test-token")
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.token != "xoxp-test-token" {
		t.Errorf("token = %q, want %q", client.token, "xoxp-test-token")
	}
}

func TestClient_ResolveChannelID(t *testing.T) {
	client := NewClient("xoxp-test")

	tests := []struct {
		input string
		want  string
	}{
		{"C01234567", "C01234567"},
		{"#engineering", "#engineering"}, // name stays as-is, resolved later via API
	}

	for _, tt := range tests {
		got := client.NormalizeChannelRef(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeChannelRef(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
