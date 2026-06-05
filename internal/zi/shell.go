package zi

const zshIntegration = `# zi shell integration
# Requires a zi binary on PATH. Install with: go install ./cmd/zi
zi() {
  local target
  case " $* " in
    *" -l "*|*" --list "*|*" -d "*|*" --delete "*|*" -m "*|*" --move "*|*" -f "*|*" --force "*|*" -r "*|*" --refresh "*|*" -s "*|*" --shell "*|*" -h "*|*" --help "*|*" refresh "*|*" shell "*)
      command zi "$@"
      return $?
      ;;
    *)
      target="$(OLDPWD="$OLDPWD" command zi "$@")" || return $?
      [[ -n "$target" ]] || return 1
      cd "$target"
      ;;
  esac
}
`
