package config

import "testing"

func TestControlCommandRetentionConfig(t *testing.T) {
	t.Setenv("CONTROL_COMMAND_RETENTION_DAYS", "14")
	t.Setenv("CONTROL_COMMAND_RETENTION_COUNT", "250")
	cfg := Load()
	if cfg.ControlCommandRetentionDays != 14 || cfg.ControlCommandRetentionCount != 250 {
		t.Fatalf("unexpected retention config: days=%d count=%d", cfg.ControlCommandRetentionDays, cfg.ControlCommandRetentionCount)
	}
	t.Setenv("CONTROL_COMMAND_RETENTION_DAYS", "invalid")
	t.Setenv("CONTROL_COMMAND_RETENTION_COUNT", "0")
	cfg = Load()
	if cfg.ControlCommandRetentionDays != DefaultControlCommandRetentionDays || cfg.ControlCommandRetentionCount != DefaultControlCommandRetentionCount {
		t.Fatalf("invalid values should use defaults: days=%d count=%d", cfg.ControlCommandRetentionDays, cfg.ControlCommandRetentionCount)
	}
}
