package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.DatabaseURL == "" || cfg.MediaBaseURL == "" || cfg.Env != "dev" {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadOverridesAndInvalidPort(t *testing.T) {
	t.Setenv("PORT", "9999")
	cfg, err := Load()
	if err != nil || cfg.Port != 9999 {
		t.Fatalf("Port = %d, err=%v, want 9999", cfg.Port, err)
	}
	t.Setenv("PORT", "abc")
	if _, err := Load(); err == nil {
		t.Fatal("want error for invalid PORT")
	}
}
