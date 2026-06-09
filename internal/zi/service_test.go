package zi

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNewMarksPostNewRunning(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	service := newTestService(t, repo, Config{
		WorktreeRelativePath: ".claude/worktrees",
		PostNew: map[string][]string{
			repo: {"printf hook > hook.txt"},
		},
	})

	path, err := service.New(ctx, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, "hook.txt")); !os.IsNotExist(err) {
		t.Fatalf("hook.txt stat error = %v, want not exist", err)
	}
	cached, err := service.cache.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cached[path].PostNewStatus != postNewRunning {
		t.Fatalf("PostNewStatus = %q, want %q", cached[path].PostNewStatus, postNewRunning)
	}
}

func TestRunPostNewRunsScripts(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	service := newTestService(t, repo, Config{
		WorktreeRelativePath: ".claude/worktrees",
		PostNew: map[string][]string{
			repo: {"printf hook > hook.txt"},
		},
	})

	path, err := service.New(ctx, "feature")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RunPostNew(ctx, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(path, "hook.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hook" {
		t.Fatalf("hook.txt = %q", string(data))
	}
	cached, err := service.cache.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cached[path].PostNewStatus != postNewDone {
		t.Fatalf("PostNewStatus = %q, want %q", cached[path].PostNewStatus, postNewDone)
	}
}

func TestNewSendsPostNewOutputToStderr(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	service := newTestService(t, repo, Config{
		WorktreeRelativePath: ".claude/worktrees",
		PostNew: map[string][]string{
			repo: {"printf hook-output"},
		},
	})

	path, err := service.New(ctx, "feature")
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr := captureStdoutStderr(t, func() {
		err = service.RunPostNew(ctx, path)
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "hook-output") {
		t.Fatalf("stderr = %q, want hook output", stderr)
	}
}

func TestRunPostNewReturnsFailure(t *testing.T) {
	ctx := context.Background()
	repo := initGitRepo(t)
	service := newTestService(t, repo, Config{
		WorktreeRelativePath: ".claude/worktrees",
		PostNew: map[string][]string{
			repo: {"exit 42"},
		},
	})

	path, err := service.New(ctx, "feature")
	if err != nil {
		t.Fatal(err)
	}
	err = service.RunPostNew(ctx, path)
	if err == nil || !strings.Contains(err.Error(), "postNew failed") {
		t.Fatalf("RunPostNew() error = %v", err)
	}
	cached, err := service.cache.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cached[path].PostNewStatus != postNewFailed {
		t.Fatalf("PostNewStatus = %q, want %q", cached[path].PostNewStatus, postNewFailed)
	}
}

func TestWithLatestPostNewStatusDoesNotRegressDone(t *testing.T) {
	service := &Service{}
	row := Worktree{
		Path:          "/repo/.claude/worktrees/feature",
		Branch:        "feature",
		PostNewStatus: postNewRunning,
	}
	latest := map[string]Worktree{
		row.Path: {
			Path:          row.Path,
			Branch:        row.Branch,
			PostNewStatus: postNewDone,
		},
	}

	got := service.withLatestPostNewStatus(row, latest)
	if got.PostNewStatus != postNewDone {
		t.Fatalf("PostNewStatus = %q, want %q", got.PostNewStatus, postNewDone)
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

func captureStdoutStderr(t *testing.T, fn func()) (string, string) {
	t.Helper()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdoutReader.Close()
	defer stderrReader.Close()
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	fn()

	if err := stdoutWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatal(err)
	}
	return string(stdout), string(stderr)
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
