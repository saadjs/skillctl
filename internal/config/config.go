package config

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Source string   `yaml:"source"`
	Tools  []string `yaml:"tools"`
}

type SkillState struct {
	Checksum string `yaml:"checksum"`
	SyncedAt string `yaml:"synced_at"`
}

type State struct {
	LastSync string                              `yaml:"last_sync"`
	Tools    map[string]map[string]SkillState    `yaml:"tools"`
}

func Dir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "skillctl")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".config", "skillctl")
	}
	return filepath.Join(home, ".config", "skillctl")
}

func ConfigPath() string {
	return filepath.Join(Dir(), "config.yaml")
}

func StatePath() string {
	return filepath.Join(Dir(), "state.yaml")
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{Tools: map[string]map[string]SkillState{}}, nil
		}
		return nil, err
	}
	var st State
	if err := yaml.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("parsing state: %w", err)
	}
	if st.Tools == nil {
		st.Tools = map[string]map[string]SkillState{}
	}
	return &st, nil
}

func SaveState(path string, st *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(st)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ChecksumSkill computes a deterministic SHA256 checksum of all files in a
// skill directory. Files are sorted by relative path, each file's content is
// hashed individually, and then "relPath:hex\n" entries are combined into a
// final hash.
func ChecksumSkill(skillPath string) (string, error) {
	type entry struct {
		rel  string
		line string
	}
	var entries []entry

	err := filepath.WalkDir(skillPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == skillPath {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(skillPath, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		mode := info.Mode()
		switch {
		case mode&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			entries = append(entries, entry{
				rel:  relSlash,
				line: fmt.Sprintf("%s:symlink:%o:%s", relSlash, mode.Perm(), filepath.ToSlash(target)),
			})
		case mode.IsDir():
			entries = append(entries, entry{
				rel:  relSlash,
				line: fmt.Sprintf("%s:dir:%o", relSlash, mode.Perm()),
			})
		case mode.IsRegular():
			hex, err := hashFile(path)
			if err != nil {
				return err
			}
			entries = append(entries, entry{
				rel:  relSlash,
				line: fmt.Sprintf("%s:file:%o:%s", relSlash, mode.Perm(), hex),
			})
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })

	combined := sha256.New()
	for _, e := range entries {
		fmt.Fprintf(combined, "%s\n", e.line)
	}
	return fmt.Sprintf("sha256:%x", combined.Sum(nil)), nil
}

func NowTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
