package skills

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Skill struct {
	Name string
	Path string
}

func Discover(skillsDir string) ([]Skill, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil, err
	}
	var result []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		skillPath := filepath.Join(skillsDir, name)
		if _, err := os.Stat(filepath.Join(skillPath, "SKILL.md")); err == nil {
			result = append(result, Skill{Name: name, Path: skillPath})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func Filter(all []Skill, names []string) ([]Skill, []string) {
	if len(names) == 0 {
		return all, nil
	}
	nameSet := map[string]bool{}
	for _, name := range names {
		nameSet[strings.TrimSpace(name)] = true
	}
	var filtered []Skill
	for _, skill := range all {
		if nameSet[skill.Name] {
			filtered = append(filtered, skill)
			delete(nameSet, skill.Name)
		}
	}
	var missing []string
	for name := range nameSet {
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return filtered, missing
}

func CopySkill(skill Skill, destRoot string, overwrite bool) error {
	dest := filepath.Join(destRoot, skill.Name)
	if _, err := os.Stat(dest); err == nil {
		if !overwrite {
			return fs.ErrExist
		}
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
	}
	return copyDir(skill.Path, dest)
}

func RemoveSkill(name, destRoot string) error {
	if name == "" {
		return errors.New("skill name required")
	}
	return os.RemoveAll(filepath.Join(destRoot, name))
}

func ListInstalled(destRoot string) ([]Skill, error) {
	entries, err := os.ReadDir(destRoot)
	if err != nil {
		return nil, err
	}
	var result []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join(destRoot, name)
		if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err == nil {
			result = append(result, Skill{Name: name, Path: path})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func copyDir(src, dest string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		mode := info.Mode()
		if d.IsDir() {
			return os.MkdirAll(target, mode.Perm())
		}
		if mode&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		destFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
		if err != nil {
			return err
		}
		defer destFile.Close()
		if _, err := io.Copy(destFile, srcFile); err != nil {
			return err
		}
		return nil
	})
}
