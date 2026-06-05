package zi

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type Printer struct {
	out io.Writer
	err io.Writer
}

func NewPrinter() Printer {
	return Printer{out: os.Stdout, err: os.Stderr}
}

func (p Printer) List(rows []Worktree, includePath bool) {
	nameWidth := 0
	branchWidth := 0
	for _, row := range rows {
		nameWidth = max(nameWidth, len(row.Name))
		branchWidth = max(branchWidth, len(row.Branch))
	}
	for _, row := range rows {
		p.Row(row, nameWidth, branchWidth, includePath)
	}
}

func (p Printer) Row(row Worktree, nameWidth int, branchWidth int, includePath bool) {
	branch := row.Branch
	if row.Merged {
		branch = ansi("9", branch)
	}
	fmt.Fprintf(p.out, "%-*s  %s", nameWidth, row.Name, branch)
	if pad := branchWidth - len(row.Branch); pad > 0 {
		fmt.Fprint(p.out, strings.Repeat(" ", pad))
	}
	if row.Dirty {
		fmt.Fprintf(p.out, "  %s", ansi("31;1", "dirty"))
	}
	if row.Ahead > 0 {
		fmt.Fprintf(p.out, "  %s", ansi("36", "ahead+"+strconv.Itoa(row.Ahead)))
	}
	if row.PRNumber != 0 {
		state := row.PRState
		if state == "" {
			state = "open"
		}
		fmt.Fprintf(p.out, "  %s", ansi(prColor(state), fmt.Sprintf("#%d %s", row.PRNumber, state)))
	}
	if includePath {
		fmt.Fprintf(p.out, "\t%s", row.Path)
	}
	fmt.Fprintln(p.out)
}

func (p Printer) Choices(rows []Worktree) string {
	var b strings.Builder
	prev := p.out
	p.out = &b
	p.List(rows, true)
	p.out = prev
	return b.String()
}

func ansi(code string, text string) string {
	if os.Getenv("NO_COLOR") != "" {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
}

func prColor(state string) string {
	switch state {
	case "draft":
		return "33"
	case "closed":
		return "31"
	case "merged":
		return "35"
	default:
		return "32"
	}
}
