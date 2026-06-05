package zi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Service struct {
	env    Env
	config Config
	git    *Git
	cache  *Cache
	prs    *PRService
}

func NewService(env Env, config Config, git *Git, cache *Cache, prs *PRService) *Service {
	return &Service{env: env, config: config, git: git, cache: cache, prs: prs}
}

func (s *Service) List(ctx context.Context) ([]Worktree, error) {
	cached, _ := s.cache.Load(ctx)
	refs, err := s.git.Worktrees(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]Worktree, 0, len(refs))
	for _, ref := range refs {
		if entry, ok := cached[ref.Path]; ok && entry.Branch == ref.Branch {
			rows = append(rows, entry)
			continue
		}
		rows = append(rows, Worktree{
			Name:    ref.Name,
			Path:    ref.Path,
			Branch:  ref.Branch,
			Display: ref.Branch,
			Dirty:   s.git.Dirty(ctx, ref.Path),
			Ahead:   s.git.Ahead(ctx, ref.Path),
		})
	}
	sortWorktrees(rows)
	return rows, nil
}

func (s *Service) Refresh(ctx context.Context) ([]Worktree, error) {
	repo, err := s.git.Repo(ctx)
	if err != nil {
		return nil, err
	}
	refs, err := s.git.Worktrees(ctx)
	if err != nil {
		return nil, err
	}

	prs := s.prs.ByBranch(ctx, repo)
	rows := make([]Worktree, 0, len(refs))
	for _, ref := range refs {
		pr := prs[ref.Branch]
		merged := s.git.IsMerged(ctx, ref.Path, ref.Branch, pr.State == "merged")
		rows = append(rows, Worktree{
			Name:     ref.Name,
			Path:     ref.Path,
			Branch:   ref.Branch,
			Display:  ref.Branch,
			PRNumber: pr.Number,
			PRState:  pr.State,
			Merged:   merged,
			Dirty:    s.git.Dirty(ctx, ref.Path),
			Ahead:    s.git.Ahead(ctx, ref.Path),
		})
	}
	sortWorktrees(rows)
	if err := s.cache.Save(ctx, rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Service) Match(ctx context.Context, query string) ([]Worktree, error) {
	rows, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	if query == "" {
		return rows, nil
	}
	matches := make([]Worktree, 0)
	for _, row := range rows {
		if strings.Contains(row.Name, query) ||
			strings.Contains(row.Branch, query) ||
			(row.PRNumber != 0 && strings.Contains(fmt.Sprintf("#%d", row.PRNumber), query)) {
			matches = append(matches, row)
		}
	}
	return matches, nil
}

func (s *Service) CacheFresh(ctx context.Context, maxAge time.Duration) bool {
	return s.cache.Fresh(ctx, maxAge)
}

func (s *Service) New(ctx context.Context, name string) (string, error) {
	repo, err := s.git.Repo(ctx)
	if err != nil {
		return "", err
	}
	root := filepath.Join(repo, s.config.WorktreeRelativePath)
	if name == "" {
		name = randomName()
	}
	if err := validateWorktreeName(name); err != nil {
		return "", err
	}
	path := filepath.Join(root, name)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("zi: worktree already exists: %s", path)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	if err := s.git.AddWorktree(ctx, repo, path, name); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) Delete(ctx context.Context, query string, force bool) error {
	if query == "" {
		return errors.New("zi: -d requires a query")
	}
	repo, err := s.git.Repo(ctx)
	if err != nil {
		return err
	}
	matches, err := s.Match(ctx, query)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("zi: no matching worktree: %s", query)
	}
	if len(matches) > 1 {
		return fmt.Errorf("zi: multiple matching worktrees: %s", query)
	}

	target := matches[0]
	if s.git.Dirty(ctx, target.Path) && !force {
		return fmt.Errorf("zi: worktree has dirty changes: %s", target.Path)
	}
	if inside(s.env.Cwd, target.Path) {
		return fmt.Errorf("zi: cannot delete current worktree from inside it: %s", target.Path)
	}

	trashDir := filepath.Join(repo, s.config.WorktreeRelativePath, ".zid-deleted")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return err
	}
	trashPath := filepath.Join(trashDir, target.Name+"."+randomSuffix())
	if err := os.Rename(target.Path, trashPath); err != nil {
		return err
	}
	s.git.Prune(ctx, repo)
	s.git.DeleteBranch(ctx, repo, target.Branch)
	fmt.Fprintf(os.Stderr, "Deleted %s\n", target.Name)
	return nil
}

func sortWorktrees(rows []Worktree) {
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})
}

func randomName() string {
	return "zi-" + randomSuffix()
}

func randomSuffix() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(b[:])
}

func inside(path string, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}
