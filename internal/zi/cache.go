package zi

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Cache struct {
	git *Git
}

type cacheFile struct {
	Entries []Worktree `json:"entries"`
}

func NewCache(git *Git) *Cache {
	return &Cache{git: git}
}

func (c *Cache) Path(ctx context.Context) (string, error) {
	root, err := c.git.WorktreeRoot(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, cacheDirectoryName, "worktrees.json"), nil
}

func (c *Cache) Load(ctx context.Context) (map[string]Worktree, error) {
	path, err := c.Path(ctx)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Worktree{}, nil
		}
		return nil, err
	}
	var file cacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		return map[string]Worktree{}, nil
	}
	entries := make(map[string]Worktree, len(file.Entries))
	for _, entry := range file.Entries {
		entries[entry.Path] = entry
	}
	return entries, nil
}

func (c *Cache) Fresh(ctx context.Context, maxAge time.Duration) bool {
	path, err := c.Path(ctx)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) < maxAge
}

func (c *Cache) Save(ctx context.Context, entries []Worktree) error {
	path, err := c.Path(ctx)
	if err != nil {
		return err
	}
	c.git.IgnoreCacheDir(ctx)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(cacheFile{Entries: entries})
	closeErr := tmp.Close()
	if err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}
	return os.Rename(tmpPath, path)
}
