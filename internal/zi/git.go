package zi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Git struct {
	env    Env
	config Config
	run    *Runner
}

type WorktreeRef struct {
	Name   string
	Path   string
	Branch string
}

func NewGit(env Env, config Config, run *Runner) *Git {
	return &Git{env: env, config: config, run: run}
}

func (g *Git) Repo(ctx context.Context) (string, error) {
	root, err := g.worktreeRootFromPath(g.env.Cwd)
	if err == nil {
		return strings.TrimSuffix(root, string(filepath.Separator)+g.config.WorktreeRelativePath), nil
	}

	top, err := g.run.Output(ctx, g.env.Cwd, "git", "rev-parse", "--show-toplevel")
	if err == nil && top != "" {
		if before, ok := g.splitAtWorktreeRoot(top); ok {
			return before, nil
		}
		return top, nil
	}

	return "", errors.New("zi: not inside a worktree repository")
}

func (g *Git) WorktreeRoot(ctx context.Context) (string, error) {
	repo, err := g.Repo(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(repo, g.config.WorktreeRelativePath), nil
}

func (g *Git) worktreeRootFromPath(start string) (string, error) {
	current := filepath.Clean(start)
	if before, ok := g.splitAtWorktreeRoot(current); ok {
		return filepath.Join(before, g.config.WorktreeRelativePath), nil
	}

	for {
		root := filepath.Join(current, g.config.WorktreeRelativePath)
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			return root, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", errors.New("worktree root not found")
}

func (g *Git) splitAtWorktreeRoot(path string) (string, bool) {
	path = filepath.Clean(path)
	needle := string(filepath.Separator) + g.config.WorktreeRelativePath
	idx := strings.Index(path, needle)
	if idx < 0 {
		return "", false
	}
	after := path[idx+len(needle):]
	if after != "" && !strings.HasPrefix(after, string(filepath.Separator)) {
		return "", false
	}
	return path[:idx], true
}

func (g *Git) WorktreePaths(ctx context.Context) ([]string, error) {
	root, err := g.WorktreeRoot(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == cacheDirectoryName {
			continue
		}
		paths = append(paths, filepath.Join(root, entry.Name()))
	}
	return paths, nil
}

func (g *Git) Worktrees(ctx context.Context) ([]WorktreeRef, error) {
	repo, err := g.Repo(ctx)
	if err != nil {
		return nil, err
	}
	root := filepath.Clean(filepath.Join(repo, g.config.WorktreeRelativePath))
	out, err := g.run.Output(ctx, repo, "git", "worktree", "list", "--porcelain")
	if err != nil {
		return g.worktreesFromDirs(ctx)
	}

	refs := make([]WorktreeRef, 0)
	var path string
	var head string
	var branch string
	flush := func() {
		if path == "" {
			return
		}
		cleanPath := filepath.Clean(path)
		if filepath.Dir(cleanPath) != root || filepath.Base(cleanPath) == cacheDirectoryName {
			path, head, branch = "", "", ""
			return
		}
		branchName := "-"
		if branch != "" {
			branchName = strings.TrimPrefix(branch, "refs/heads/")
		} else if head != "" {
			short := head
			if len(short) > 12 {
				short = short[:12]
			}
			branchName = "detached:" + short
		}
		refs = append(refs, WorktreeRef{
			Name:   filepath.Base(cleanPath),
			Path:   cleanPath,
			Branch: branchName,
		})
		path, head, branch = "", "", ""
	}

	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			flush()
			continue
		}
		key, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		switch key {
		case "worktree":
			flush()
			path = value
		case "HEAD":
			head = value
		case "branch":
			branch = value
		}
	}
	flush()
	return refs, nil
}

func (g *Git) worktreesFromDirs(ctx context.Context) ([]WorktreeRef, error) {
	paths, err := g.WorktreePaths(ctx)
	if err != nil {
		return nil, err
	}
	refs := make([]WorktreeRef, 0, len(paths))
	for _, path := range paths {
		refs = append(refs, WorktreeRef{
			Name:   filepath.Base(path),
			Path:   path,
			Branch: g.Branch(ctx, path),
		})
	}
	return refs, nil
}

func (g *Git) Branch(ctx context.Context, path string) string {
	branch, err := g.run.Output(ctx, path, "git", "branch", "--show-current")
	if err == nil && branch != "" {
		return branch
	}
	head, err := g.run.Output(ctx, path, "git", "rev-parse", "--short", "HEAD")
	if err == nil && head != "" {
		return "detached:" + head
	}
	return "-"
}

func (g *Git) Dirty(ctx context.Context, path string) bool {
	out, err := g.run.Output(ctx, path, "git", "status", "--porcelain", "--untracked-files=normal")
	return err == nil && out != ""
}

func (g *Git) MainRef(ctx context.Context, path string) string {
	if _, err := g.run.Output(ctx, path, "git", "rev-parse", "--verify", "--quiet", "origin/main"); err == nil {
		return "origin/main"
	}
	if _, err := g.run.Output(ctx, path, "git", "rev-parse", "--verify", "--quiet", "main"); err == nil {
		return "main"
	}
	return ""
}

func (g *Git) Ahead(ctx context.Context, path string) int {
	ref := g.MainRef(ctx, path)
	if ref == "" {
		return 0
	}
	out, err := g.run.Output(ctx, path, "git", "rev-list", "--count", ref+"..HEAD")
	if err != nil {
		return 0
	}
	count, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return count
}

func (g *Git) IsMerged(ctx context.Context, path string, branch string, prMerged bool) bool {
	if prMerged {
		return true
	}
	if branch == "" || branch == "-" || strings.HasPrefix(branch, "detached:") {
		return false
	}
	ref := g.MainRef(ctx, path)
	if ref == "" {
		return false
	}
	if err := g.run.Run(ctx, path, "git", "merge-base", "--is-ancestor", "HEAD", ref); err == nil {
		return true
	}
	remaining, err := g.run.Output(ctx, path, "git", "log", "--format=%H", "--cherry-pick", "--right-only", "--no-merges", ref+"...HEAD")
	return err == nil && strings.TrimSpace(remaining) == ""
}

func (g *Git) BranchExists(ctx context.Context, repo string, name string) bool {
	return g.run.Run(ctx, repo, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+name) == nil
}

func (g *Git) AddWorktree(ctx context.Context, repo string, path string, name string) error {
	if g.BranchExists(ctx, repo, name) {
		_, err := g.run.Output(ctx, repo, "git", "worktree", "add", path, name)
		return err
	}
	ref := "origin/main"
	if _, err := g.run.Output(ctx, repo, "git", "rev-parse", "--verify", "--quiet", ref); err != nil {
		ref = "main"
	}
	_, err := g.run.Output(ctx, repo, "git", "worktree", "add", "-b", name, path, ref)
	return err
}

func (g *Git) MoveWorktree(ctx context.Context, repo string, oldPath string, newPath string) error {
	_, err := g.run.Output(ctx, repo, "git", "worktree", "move", oldPath, newPath)
	return err
}

func (g *Git) RenameCurrentBranch(ctx context.Context, path string, name string) error {
	_, err := g.run.Output(ctx, path, "git", "branch", "-m", name)
	return err
}

func (g *Git) Prune(ctx context.Context, repo string) {
	_, _ = g.run.Output(ctx, repo, "git", "worktree", "prune")
}

func (g *Git) DeleteBranch(ctx context.Context, repo string, branch string) {
	if branch == "" || branch == "-" || strings.HasPrefix(branch, "detached:") {
		return
	}
	_, _ = g.run.Output(ctx, repo, "git", "branch", "-d", branch)
}

func (g *Git) IgnoreCacheDir(ctx context.Context) {
	repo, err := g.Repo(ctx)
	if err != nil {
		return
	}
	excludePath, err := g.run.Output(ctx, repo, "git", "rev-parse", "--git-path", "info/exclude")
	if err != nil || excludePath == "" {
		return
	}
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(repo, excludePath)
	}

	pattern := filepath.ToSlash(filepath.Join(g.config.WorktreeRelativePath, cacheDirectoryName)) + "/"
	data, _ := os.ReadFile(excludePath)
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == pattern {
			return
		}
	}
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(excludePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
		_, _ = file.WriteString("\n")
	}
	_, _ = file.WriteString("# zi cache\n" + pattern + "\n")
}

func validateWorktreeName(name string) error {
	if name == "" {
		return errors.New("empty worktree name")
	}
	if strings.HasPrefix(name, "-") || strings.Contains(name, "/") {
		return fmt.Errorf("invalid worktree name: %s", name)
	}
	return nil
}
