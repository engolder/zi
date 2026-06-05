package zi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	defaultWorktreeRelativePath = ".claude/worktree"
	cacheDirectoryName          = "zi-cache"
)

type Config struct {
	WorktreeRelativePath string `json:"worktreeRelativePath"`
}

func NewConfig(env Env) (Config, error) {
	config := Config{WorktreeRelativePath: defaultWorktreeRelativePath}

	path := env.ConfigPath()
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return Config{}, fmt.Errorf("zi: invalid config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return Config{}, err
	}

	if value := os.Getenv("ZI_WORKTREE_RELATIVE_PATH"); value != "" {
		config.WorktreeRelativePath = value
	}
	config.WorktreeRelativePath = filepath.Clean(config.WorktreeRelativePath)
	if filepath.IsAbs(config.WorktreeRelativePath) || config.WorktreeRelativePath == "." {
		return Config{}, fmt.Errorf("zi: worktreeRelativePath must be a relative path: %s", config.WorktreeRelativePath)
	}
	return config, nil
}
