package zi

import "time"

type Worktree struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Branch  string `json:"branch"`
	Display string `json:"display"`
	Root    bool   `json:"root,omitempty"`

	PRNumber int    `json:"pr_number,omitempty"`
	PRState  string `json:"pr_state,omitempty"`
	Merged   bool   `json:"merged,omitempty"`
	Dirty    bool   `json:"dirty,omitempty"`
	Ahead    int    `json:"ahead,omitempty"`

	PostNewStatus string `json:"post_new_status,omitempty"`
	PostNewError  string `json:"post_new_error,omitempty"`
}

type PR struct {
	Number      int
	State       string
	HeadRefName string
	IsDraft     bool
	MergedAt    string
	UpdatedAt   time.Time
}
