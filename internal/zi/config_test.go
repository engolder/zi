package zi

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestNewConfigParsesPostNew(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	writeConfig(t, home, `{
		"worktreeRelativePath": ".worktrees",
		"postNew": {
			`+strconv.Quote(repo)+`: ["yarn install"]
		}
	}`)

	config, err := NewConfig(Env{Home: home})
	if err != nil {
		t.Fatal(err)
	}

	if config.WorktreeRelativePath != ".worktrees" {
		t.Fatalf("WorktreeRelativePath = %q", config.WorktreeRelativePath)
	}
	if got := config.PostNewScripts(repo); !reflect.DeepEqual(got, []string{"yarn install"}) {
		t.Fatalf("PostNewScripts() = %#v", got)
	}
}

func TestNewConfigRejectsRelativePostNewPath(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, `{
		"postNew": {
			"place-frontend": ["yarn install"]
		}
	}`)

	_, err := NewConfig(Env{Home: home})
	if err == nil || !strings.Contains(err.Error(), "postNew repository path must be absolute") {
		t.Fatalf("NewConfig() error = %v", err)
	}
}

func writeConfig(t *testing.T, home string, content string) {
	t.Helper()
	configDir := filepath.Join(home, ".config", "zi")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
