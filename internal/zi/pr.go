package zi

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

type PRService struct {
	git *Git
	run *Runner
}

type ghPR struct {
	Number      int    `json:"number"`
	State       string `json:"state"`
	MergedAt    string `json:"mergedAt"`
	HeadRefName string `json:"headRefName"`
	IsDraft     bool   `json:"isDraft"`
	UpdatedAt   string `json:"updatedAt"`
	CreatedAt   string `json:"createdAt"`
}

func NewPRService(git *Git, run *Runner) *PRService {
	return &PRService{git: git, run: run}
}

func (p *PRService) ByBranch(ctx context.Context, repo string) map[string]PR {
	if _, err := exec.LookPath("gh"); err != nil {
		return map[string]PR{}
	}
	out, err := p.run.Output(ctx, repo, "gh", "pr", "list", "--state", "all", "--limit", "200", "--json", "number,state,mergedAt,headRefName,isDraft,updatedAt,createdAt")
	if err != nil || out == "" {
		return map[string]PR{}
	}

	var raw []ghPR
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return map[string]PR{}
	}

	byBranch := map[string]PR{}
	for _, item := range raw {
		updated := parsePRTime(item.UpdatedAt)
		if updated.IsZero() {
			updated = parsePRTime(item.CreatedAt)
		}
		pr := PR{
			Number:      item.Number,
			State:       prState(item.State, item.MergedAt, item.IsDraft),
			HeadRefName: item.HeadRefName,
			IsDraft:     item.IsDraft,
			MergedAt:    item.MergedAt,
			UpdatedAt:   updated,
		}
		current, ok := byBranch[item.HeadRefName]
		if !ok || pr.UpdatedAt.After(current.UpdatedAt) {
			byBranch[item.HeadRefName] = pr
		}
	}
	return byBranch
}

func prState(state string, mergedAt string, draft bool) string {
	if mergedAt != "" {
		return "merged"
	}
	if state == "OPEN" && draft {
		return "draft"
	}
	if state == "OPEN" {
		return "open"
	}
	return "closed"
}

func parsePRTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
