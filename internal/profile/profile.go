package profile

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Profile struct {
	Name string `yaml:"name"`
	DSN  string `yaml:"dsn"`
}

func configDir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d, nil
	}
	return os.UserConfigDir()
}

func profilePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", fmt.Errorf("finding user config dir: %w", err)
	}
	return filepath.Join(dir, "asql", "profiles.yaml"), nil
}

func Load() ([]Profile, error) {
	path, err := profilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading profiles: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	var profiles []Profile
	if err := yaml.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("parsing profiles: %w", err)
	}
	return profiles, nil
}

func Save(profiles []Profile) error {
	path, err := profilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(profiles)
	if err != nil {
		return fmt.Errorf("marshaling profiles: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing profiles: %w", err)
	}
	return nil
}

// Find returns the profile with the given name, or nil if not found.
func Find(profiles []Profile, name string) *Profile {
	for i := range profiles {
		if profiles[i].Name == name {
			return &profiles[i]
		}
	}
	return nil
}
