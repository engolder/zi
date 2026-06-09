package zi

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func TestPlanDeleteWithoutQueryTargetsCurrentWorktree(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	config := Config{WorktreeRelativePath: ".claude/worktrees"}
	service := newTestService(t, repo, config)

	path, err := service.New(ctx, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service.env.Cwd = filepath.Join(path, "nested")
	if err := os.MkdirAll(service.env.Cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	plan, err := service.PlanDelete(ctx, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Confirm {
		t.Fatal("PlanDelete() Confirm = false")
	}
	if plan.NextPath != repo {
		t.Fatalf("PlanDelete() NextPath = %q, want %q", plan.NextPath, repo)
	}
	if plan.Target.Path != path {
		t.Fatalf("PlanDelete() target path = %q, want %q", plan.Target.Path, path)
	}
	if !plan.Target.Dirty {
		t.Fatal("PlanDelete() target Dirty = false")
	}

	if err := service.Delete(ctx, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("deleted worktree stat error = %v, want not exist", err)
	}
}

func TestPlanDeleteWithoutQueryRequiresCurrentWorktree(t *testing.T) {
	repo := initGitRepo(t)
	service := newTestService(t, repo, Config{WorktreeRelativePath: ".claude/worktrees"})

	_, err := service.PlanDelete(context.Background(), "", false)
	if err == nil || !strings.Contains(err.Error(), "must run inside a worktree") {
		t.Fatalf("PlanDelete() error = %v", err)
	}
}

func TestDeleteWithoutQueryCancelDoesNotDelete(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	config := Config{WorktreeRelativePath: ".claude/worktrees"}
	setup := newTestService(t, repo, config)
	path, err := setup.New(ctx, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run := NewRunner()
	env := Env{Home: t.TempDir(), Cwd: path}
	git := NewGit(env, config, run)
	service := NewService(env, config, git, NewCache(git), NewPRService(git, run), run)
	cli := NewCLI(service, run)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cli.print = Printer{out: &stdout, err: &stderr}
	fzfArgsPath := fakeFzf(t, "cancel")

	code, err := cli.Run(ctx, []string{"-d"})
	if code != 1 {
		t.Fatalf("Run() code = %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "delete canceled") {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	fzfArgs, err := os.ReadFile(fzfArgsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fzfArgs), "Delete worktree?") || !strings.Contains(string(fzfArgs), "feature") || !strings.Contains(string(fzfArgs), "dirty") {
		t.Fatalf("fzf args = %q, want confirmation with target status", string(fzfArgs))
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("worktree stat error = %v", err)
	}
}

func TestPlanPruneTargetsOnlyCleanMergedWorktrees(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	config := Config{WorktreeRelativePath: ".claude/worktrees"}
	service := newTestService(t, repo, config)

	merged, err := service.New(ctx, "merged")
	if err != nil {
		t.Fatal(err)
	}
	dirty, err := service.New(ctx, "dirty")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirty, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ahead, err := service.New(ctx, "ahead")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ahead, "ahead.txt"), []byte("ahead\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, ahead, "add", "ahead.txt")
	runGit(t, ahead, "commit", "-m", "ahead")
	detached, err := service.New(ctx, "detached")
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, detached, "checkout", "--detach")
	current, err := service.New(ctx, "current")
	if err != nil {
		t.Fatal(err)
	}
	service.env.Cwd = filepath.Join(current, "nested")
	if err := os.MkdirAll(service.env.Cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	plan, err := service.PlanPrune(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Repo != repo {
		t.Fatalf("PlanPrune() repo = %q, want %q", plan.Repo, repo)
	}
	if len(plan.Targets) != 1 {
		t.Fatalf("PlanPrune() targets = %#v, want exactly one target", plan.Targets)
	}
	if plan.Targets[0].Path != merged {
		t.Fatalf("PlanPrune() target path = %q, want %q", plan.Targets[0].Path, merged)
	}
}

func TestPruneDeletesTargets(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	config := Config{WorktreeRelativePath: ".claude/worktrees"}
	service := newTestService(t, repo, config)

	path, err := service.New(ctx, "merged")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanPrune(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Targets) != 1 {
		t.Fatalf("PlanPrune() targets = %#v, want one target", plan.Targets)
	}

	if err := service.Prune(ctx, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("pruned worktree stat error = %v, want not exist", err)
	}
	if service.git.BranchExists(ctx, repo, "merged") {
		t.Fatal("branch merged still exists after prune")
	}
}

func TestPruneCancelDoesNotDelete(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	config := Config{WorktreeRelativePath: ".claude/worktrees"}
	setup := newTestService(t, repo, config)
	path, err := setup.New(ctx, "merged")
	if err != nil {
		t.Fatal(err)
	}

	run := NewRunner()
	env := Env{Home: t.TempDir(), Cwd: repo}
	git := NewGit(env, config, run)
	service := NewService(env, config, git, NewCache(git), NewPRService(git, run), run)
	cli := NewCLI(service, run)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cli.print = Printer{out: &stdout, err: &stderr}
	fzfArgsPath := fakeFzf(t, "cancel")

	code, err := cli.Run(ctx, []string{"--prune"})
	if code != 1 {
		t.Fatalf("Run() code = %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "prune canceled") {
		t.Fatalf("Run() error = %v", err)
	}
	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	fzfArgs, err := os.ReadFile(fzfArgsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fzfArgs), "Prune clean merged worktrees?") || !strings.Contains(string(fzfArgs), "merged") {
		t.Fatalf("fzf args = %q, want confirmation with prune target", string(fzfArgs))
	}
	if !strings.Contains(string(fzfArgs), "--stdin--\nprune\ncancel\n") {
		t.Fatalf("fzf input = %q, want prune before cancel", string(fzfArgs))
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("worktree stat error = %v", err)
	}
}

func fakeFzf(t *testing.T, selected string) string {
	t.Helper()
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	script := "#!/bin/sh\n{\nprintf '%s\\n' \"$@\"\nprintf '%s\\n' --stdin--\ncat\n} > " + strconv.Quote(argsPath) + "\nprintf '%s\\n' " + strconv.Quote(selected) + "\n"
	path := filepath.Join(dir, "fzf")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath
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
