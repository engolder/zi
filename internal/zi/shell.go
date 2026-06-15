package zi

const zshIntegration = `# zi shell integration
# Requires a zi binary on PATH. Install with: go install ./cmd/zi
zi() {
  local target
  case " $* " in
    *" -m "*|*" --move "*)
      target="$(command zi "$@")" || return $?
      if [[ -n "$target" && ! -d "$PWD" ]]; then
        cd "$target"
      fi
      return 0
      ;;
    *" -l "*|*" --list "*|*" -s "*|*" --shell "*|*" -h "*|*" --help "*|*" refresh "*|*" shell "*)
      command zi "$@"
      return $?
      ;;
    *" -d "*|*" --delete "*)
      target="$(command zi "$@")" || return $?
      if [[ -n "$target" ]]; then
        cd "$target"
        return $?
      fi
      return 0
      ;;
    *" -f "*|*" --force "*|*" -r "*|*" --refresh "*)
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
