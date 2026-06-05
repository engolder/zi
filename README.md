# zi

Worktree picker for repositories that keep temporary worktrees under a configurable repo-relative directory.

## Commands

```sh
zi [query]       # pick a worktree and print its path
zi -l, --list    # list worktrees
zi -n, --new     # create a worktree
zi -d, --delete  # delete a clean worktree
zi -f, --force   # allow deleting dirty worktrees with --delete
zi -r, --refresh # refresh cache before running
zi -s, --shell   # print shell integration
zi -h, --help    # show help
zi refresh       # refresh cache
zi --shell zsh   # print zsh integration
```

## Config

Default config:

```json
{
  "worktreeRelativePath": ".claude/worktree"
}
```

Config file path:

```sh
~/.config/zi/config.json
```

`ZI_WORKTREE_RELATIVE_PATH` overrides the config file for one command.

The cache is stored inside the configured worktree root:

```sh
<repo>/<worktreeRelativePath>/zi-cache/worktrees.json
```

The binary cannot change the parent shell's working directory. Use the zsh integration for `cd` behavior:

```sh
eval "$(zi --shell zsh)"
```
