package zi

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRunsPostNewScripts(t *testing.T) {
	repo := initGitRepo(t)
	service := newTestService(t, repo, Config{
		WorktreeRelativePath: ".claude/worktrees",
		PostNew: map[string][]string{
			repo: {"printf hook > hook.txt"},
		},
	})

	path, err := service.New(context.Background(), "feature")
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(path, "hook.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hook" {
		t.Fatalf("hook.txt = %q", string(data))
	}
}

func TestNewReturnsPostNewFailure(t *testing.T) {
	repo := initGitRepo(t)
	service := newTestService(t, repo, Config{
		WorktreeRelativePath: ".claude/worktrees",
		PostNew: map[string][]string{
			repo: {"exit 42"},
		},
	})

	_, err := service.New(context.Background(), "feature")
	if err == nil || !strings.Contains(err.Error(), "postNew failed") {
		t.Fatalf("New() error = %v", err)
	}
}

func newTestService(t *testing.T, repo string, config Config) *Service {
	t.Helper()
	run := NewRunner()
	env := Env{Home: t.TempDir(), Cwd: repo}
	git := NewGit(env, config, run)
	return NewService(env, config, git, NewCache(git), NewPRService(git, run), run)
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	runGit(t, repo, "init")
	runGit(t, repo, "checkout", "-b", "main")
	runGit(t, repo, "config", "user.email", "zi@example.com")
	runGit(t, repo, "config", "user.name", "zi")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "init")

	return cleanRepoPath(repo)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}
