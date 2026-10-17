package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env")
	body := "# settings\nORIEL_TEST_A=plain\n\nORIEL_TEST_B=\"quoted value\"\nORIEL_TEST_C=has=equals\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"ORIEL_TEST_A": "plain",
		"ORIEL_TEST_B": "quoted value",
		"ORIEL_TEST_C": "has=equals",
	} {
		if got := os.Getenv(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestLoadEnvFileRejectsBadLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "env")
	if err := os.WriteFile(path, []byte("NOT A SETTING\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnvFile(path); err == nil {
		t.Fatal("expected an error for a line without =")
	}
}

func TestFromEnvNamesWhatIsMissing(t *testing.T) {
	for _, key := range []string{"ORIEL_ADDR", "ORIEL_DATA", "ORIEL_SITE_URL", "ORIEL_SECRET", "ORIEL_DESK_KEY"} {
		t.Setenv(key, "")
	}
	_, err := FromEnv()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, key := range []string{"ORIEL_ADDR", "ORIEL_DATA", "ORIEL_SITE_URL", "ORIEL_SECRET", "ORIEL_DESK_KEY"} {
		if !contains(err.Error(), key) {
			t.Errorf("error %q does not name %s", err, key)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
