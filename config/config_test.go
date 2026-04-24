package config

import (
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := Load("../testdata/config.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Token != "xoxp-test-token" {
		t.Errorf("Token = %q, want %q", cfg.Token, "xoxp-test-token")
	}

	if len(cfg.Channels) != 2 {
		t.Errorf("Channels count = %d, want 2", len(cfg.Channels))
	}

	if cfg.Settings.PollInterval != 3*time.Second {
		t.Errorf("PollInterval = %v, want 3s", cfg.Settings.PollInterval)
	}

	if !cfg.DMs {
		t.Error("DMs should be true")
	}

	if !cfg.Sounds.Enabled {
		t.Error("Sounds.Enabled should be true")
	}

	if cfg.Status.Text != "Work Session In Progress - DM to Reach Me" {
		t.Errorf("Status.Text = %q, want work session text", cfg.Status.Text)
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	t.Setenv("SLACK_TOKEN", "xoxp-env-token")

	cfg, err := Load("../testdata/config.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Token != "xoxp-env-token" {
		t.Errorf("Token = %q, want env override %q", cfg.Token, "xoxp-env-token")
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := Load("nonexistent.yaml")
	if err == nil {
		t.Error("Expected error for missing file")
	}
}
