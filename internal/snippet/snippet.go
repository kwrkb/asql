package snippet

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kwrkb/asql/internal/fsutil"
	"gopkg.in/yaml.v3"
)

type Snippet struct {
	Name  string `yaml:"name"`
	Query string `yaml:"query"`
}

func configDir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d, nil
	}
	return os.UserConfigDir()
}

func snippetPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", fmt.Errorf("finding user config dir: %w", err)
	}
	return filepath.Join(dir, "asql", "snippets.yaml"), nil
}

func Load() ([]Snippet, error) {
	path, err := snippetPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading snippets: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	var snippets []Snippet
	if err := yaml.Unmarshal(data, &snippets); err != nil {
		return nil, fmt.Errorf("parsing snippets: %w", err)
	}
	return snippets, nil
}

func Save(snippets []Snippet) error {
	path, err := snippetPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := yaml.Marshal(snippets)
	if err != nil {
		return fmt.Errorf("marshaling snippets: %w", err)
	}

	if err := fsutil.AtomicWrite(path, data, 0o600); err != nil {
		return fmt.Errorf("writing snippets: %w", err)
	}
	return nil
}

