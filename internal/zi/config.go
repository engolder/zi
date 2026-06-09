package zi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultWorktreeRelativePath = ".claude/worktree"
	cacheDirectoryName          = "zi-cache"
)

type Config struct {
	WorktreeRelativePath string              `json:"worktreeRelativePath"`
	PostNew              map[string][]string `json:"postNew,omitempty"`
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
	postNew := make(map[string][]string, len(config.PostNew))
	for repo, scripts := range config.PostNew {
		repo = filepath.Clean(repo)
		if !filepath.IsAbs(repo) {
			return Config{}, fmt.Errorf("zi: postNew repository path must be absolute: %s", repo)
		}
		repo = cleanRepoPath(repo)
		for _, script := range scripts {
			if strings.TrimSpace(script) == "" {
				return Config{}, fmt.Errorf("zi: postNew script must not be empty: %s", repo)
			}
		}
		postNew[repo] = scripts
	}
	config.PostNew = postNew
	return config, nil
}

func (c Config) PostNewScripts(repo string) []string {
	return c.PostNew[cleanRepoPath(repo)]
}

func cleanRepoPath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}
