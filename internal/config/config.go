// Package config reads the server's settings from its environment.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// Config holds everything the server needs to know about where it runs.
type Config struct {
	// Addr is the local address the server listens on, such as 127.0.0.1:8400.
	Addr string
	// DataDir holds the sample database and every visitor's copy.
	DataDir string
	// SiteURL is the public address, without a trailing slash.
	SiteURL string
	// Secret signs cookies and form tokens.
	Secret []byte
	// DeskKey signs the one-click links into the sales desk.
	DeskKey []byte
}

// FromEnv reads the settings from the process environment.
func FromEnv() (Config, error) {
	c := Config{
		Addr:    os.Getenv("ORIEL_ADDR"),
		DataDir: os.Getenv("ORIEL_DATA"),
		SiteURL: strings.TrimRight(os.Getenv("ORIEL_SITE_URL"), "/"),
		Secret:  []byte(os.Getenv("ORIEL_SECRET")),
		DeskKey: []byte(os.Getenv("ORIEL_DESK_KEY")),
	}
	var missing []string
	if c.Addr == "" {
		missing = append(missing, "ORIEL_ADDR")
	}
	if c.DataDir == "" {
		missing = append(missing, "ORIEL_DATA")
	}
	if c.SiteURL == "" {
		missing = append(missing, "ORIEL_SITE_URL")
	}
	if len(c.Secret) < 32 {
		missing = append(missing, "ORIEL_SECRET (at least 32 characters)")
	}
	if len(c.DeskKey) < 16 {
		missing = append(missing, "ORIEL_DESK_KEY (at least 16 characters)")
	}
	if len(missing) > 0 {
		return c, fmt.Errorf("missing settings: %s", strings.Join(missing, ", "))
	}
	if u, err := url.Parse(c.SiteURL); err != nil || u.Scheme == "" || u.Host == "" {
		return c, errors.New("ORIEL_SITE_URL must be an absolute address")
	}
	return c, nil
}

// LoadEnvFile sets the variables named in a KEY=VALUE file, one per line,
// without passing the file through a shell. Lines starting with # are skipped,
// and a value may be wrapped in double quotes.
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, ok := strings.Cut(text, "=")
		if !ok || key == "" {
			return fmt.Errorf("%s line %d is not KEY=VALUE", path, line)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = value[1 : len(value)-1]
		}
		if err := os.Setenv(strings.TrimSpace(key), value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
