// Package config resolves the GLKJ harness home, profile and model settings.
package config

import (
	"os"
	"path/filepath"
)

// DefaultBaseURL is the GLKJ placeholder gateway; an operator must point
// GLKJ_BASE_URL at the real one.
const DefaultBaseURL = "https://api.glkj.example.com"

// DefaultProfile is used when neither a flag nor a manifest names one.
const DefaultProfile = "web"

// Config is the resolved process configuration.
type Config struct {
	Home    string
	Profile string
	APIKey  string
	BaseURL string
	Model   string
	WebDist string
	RepoDir string
	Anchor  string
}

// Load resolves configuration from flags and the environment.
func Load(profile string) Config {
	home := os.Getenv("GLH_HOME")
	if home == "" {
		if userHome, err := os.UserHomeDir(); err == nil {
			home = filepath.Join(userHome, ".glh")
		} else {
			home = ".glh"
		}
	}
	baseURL := os.Getenv("GLKJ_BASE_URL")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if profile == "" {
		profile = DefaultProfile
	}
	return Config{
		Home:    home,
		Profile: profile,
		APIKey:  os.Getenv("GLKJ_API_KEY"),
		BaseURL: baseURL,
		Model:   os.Getenv("GLH_MODEL"),
		WebDist: os.Getenv("GLH_WEB_DIST"),
	}
}

// InstallAnchor is the directory whose node_modules tree holds the bundle
// packages: GLH_INSTALL_ANCHOR, else <repo>/apps/cli, else the working
// directory.
func (c Config) InstallAnchor() string {
	if c.Anchor != "" {
		return c.Anchor
	}
	if anchor := os.Getenv("GLH_INSTALL_ANCHOR"); anchor != "" {
		return anchor
	}
	if c.RepoDir != "" {
		return filepath.Join(c.RepoDir, "apps", "cli")
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

// ProfilePath returns the profile directory under the harness home.
func (c Config) ProfilePath() string {
	return filepath.Join(c.Home, "profiles", c.Profile)
}
