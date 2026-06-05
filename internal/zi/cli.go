package zi

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const backgroundRefreshDebounce = 5 * time.Second

type CLI struct {
	service *Service
	run     *Runner
	print   Printer
}

func NewCLI(service *Service, run *Runner) *CLI {
	return &CLI{service: service, run: run, print: NewPrinter()}
}

func (c *CLI) Run(ctx context.Context, args []string) (int, error) {
	if hasHelp(args) {
		usage()
		return 0, nil
	}

	if len(args) > 0 {
		switch args[0] {
		case "-":
			if len(args) != 1 {
				return 2, errors.New("zi: - does not accept arguments")
			}
			previous := os.Getenv("OLDPWD")
			if previous == "" {
				return 1, errors.New("zi: OLDPWD is not set")
			}
			fmt.Println(previous)
			return 0, nil
		case "refresh":
			_, err := c.service.Refresh(ctx)
			return code(err), err
		case "shell":
			shell := "zsh"
			if len(args) > 1 {
				shell = args[1]
			}
			err := c.shell(shell)
			return code(err), err
		}
	}

	fs := flag.NewFlagSet("zi", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var list bool
	var newWorktree bool
	var deleteWorktree bool
	var moveWorktree bool
	var force bool
	var refresh bool
	var shell string
	fs.BoolVar(&list, "l", false, "list worktrees")
	fs.BoolVar(&list, "list", false, "list worktrees")
	fs.BoolVar(&newWorktree, "n", false, "create a worktree")
	fs.BoolVar(&newWorktree, "new", false, "create a worktree")
	fs.BoolVar(&deleteWorktree, "d", false, "delete a worktree")
	fs.BoolVar(&deleteWorktree, "delete", false, "delete a worktree")
	fs.BoolVar(&moveWorktree, "m", false, "move a worktree")
	fs.BoolVar(&moveWorktree, "move", false, "move a worktree")
	fs.BoolVar(&force, "f", false, "force delete dirty worktrees")
	fs.BoolVar(&force, "force", false, "force delete dirty worktrees")
	fs.BoolVar(&refresh, "r", false, "refresh status and PR cache")
	fs.BoolVar(&refresh, "refresh", false, "refresh status and PR cache")
	fs.StringVar(&shell, "s", "", "print shell integration")
	fs.StringVar(&shell, "shell", "", "print shell integration")
	fs.Usage = usage
	if err := fs.Parse(args); err != nil {
		return 2, err
	}

	if shell != "" {
		err := c.shell(shell)
		return code(err), err
	}

	rest := fs.Args()
	if force && !deleteWorktree {
		return 2, errors.New("zi: -f/--force is only valid with -d/--delete")
	}
	if refresh {
		if _, err := c.service.Refresh(ctx); err != nil {
			return 1, err
		}
		if !list && !newWorktree && !deleteWorktree && !moveWorktree && len(rest) == 0 {
			return 0, nil
		}
	}

	switch {
	case list:
		rows, err := c.service.List(ctx)
		if err != nil {
			return 1, err
		}
		if !refresh {
			c.refreshInBackground(ctx, false)
		}
		c.print.List(rows, false)
		return 0, nil
	case newWorktree:
		name := ""
		if len(rest) > 0 {
			name = rest[0]
		}
		path, err := c.service.New(ctx, name)
		if err != nil {
			return 1, err
		}
		c.refreshInBackground(ctx, true)
		fmt.Println(path)
		return 0, nil
	case deleteWorktree:
		query := ""
		if len(rest) > 0 {
			query = rest[0]
		}
		err := c.service.Delete(ctx, query, force)
		if err == nil {
			c.refreshInBackground(ctx, true)
		}
		return code(err), err
	case moveWorktree:
		if len(rest) != 2 {
			return 2, errors.New("zi: -m/--move requires a query and name")
		}
		path, err := c.service.Move(ctx, rest[0], rest[1])
		if err != nil {
			return 1, err
		}
		c.refreshInBackground(ctx, true)
		fmt.Println(path)
		return 0, nil
	default:
		query := ""
		if len(rest) > 0 {
			query = rest[0]
		}
		path, err := c.pick(ctx, query)
		if err != nil {
			return code(err), err
		}
		if !refresh {
			c.refreshInBackground(ctx, false)
		}
		fmt.Println(path)
		return 0, nil
	}
}

func (c *CLI) pick(ctx context.Context, query string) (string, error) {
	matches, err := c.service.Match(ctx, query)
	if err != nil {
		return "", err
	}
	if query != "" {
		switch len(matches) {
		case 0:
			return "", fmt.Errorf("zi: no matching worktree: %s", query)
		case 1:
			return matches[0].Path, nil
		default:
			return "", fmt.Errorf("zi: multiple matching worktrees: %s", query)
		}
	}
	if len(matches) == 0 {
		return "", errors.New("zi: no worktrees")
	}
	if _, err := exec.LookPath("fzf"); err != nil {
		return "", errors.New("zi: fzf not found")
	}
	selected, err := c.run.RunInput(ctx, c.print.Choices(matches), "", "fzf", "--ansi", "--height", "40%", "--layout=reverse", "--prompt=zi> ", "--delimiter=\t", "--with-nth=1")
	if err != nil {
		return "", err
	}
	if selected == "" {
		return "", errors.New("zi: no selection")
	}
	parts := strings.Split(selected, "\t")
	return parts[len(parts)-1], nil
}

func (c *CLI) shell(shell string) error {
	if shell != "zsh" {
		return fmt.Errorf("zi: unsupported shell: %s", shell)
	}
	fmt.Print(zshIntegration)
	return nil
}

func (c *CLI) refreshInBackground(ctx context.Context, force bool) {
	if !force && c.service.CacheFresh(ctx, backgroundRefreshDebounce) {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "refresh")
	if err := cmd.Start(); err != nil {
		return
	}
	_ = cmd.Process.Release()
}

func hasHelp(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func code(err error) int {
	if err != nil {
		return 1
	}
	return 0
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage:
  zi [query]          pick a worktree and print its path
  zi -                pick the previous directory
  zi -l, --list       list worktrees
  zi -n, --new [name] create a worktree and print its path
  zi -d, --delete     delete a worktree
  zi -m, --move <query> <name>
                     move a worktree and rename its branch
  zi -f, --force      allow deleting dirty worktrees with --delete
  zi -r, --refresh    refresh status/PR cache before running
  zi -s, --shell zsh  print shell integration
  zi -h, --help       show this help

Commands:
  zi refresh          refresh status/PR cache
  zi shell [zsh]      print shell integration

`)
}
