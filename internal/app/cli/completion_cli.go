package cli

import (
	"fmt"
	"io"
	"strings"
)

func runCompletionCommand(args []string, stdout io.Writer) error {
	if isHelp(args) {
		printCompletionHelp(stdout)
		return nil
	}
	if len(args) != 1 {
		return usageError("completion requires exactly one shell: bash, zsh, or fish")
	}

	switch strings.ToLower(args[0]) {
	case "bash":
		printBashCompletion(stdout)
	case "zsh":
		printZshCompletion(stdout)
	case "fish":
		printFishCompletion(stdout)
	default:
		return usageError("unsupported completion shell %q", args[0])
	}
	return nil
}

func printCompletionHelp(w io.Writer) {
	fmt.Fprint(w, `Usage:
  tunwarden completion bash
  tunwarden completion zsh
  tunwarden completion fish

Generate shell completion definitions for stdout. The command is read-only: it
prints completion scripts and does not contact tunwardend, start Xray, mutate
networking, or require root. Bash, zsh, and fish completion may read local
profile and subscription IDs during interactive completion only.
`)
}

func printBashCompletion(w io.Writer) {
	fmt.Fprintf(w, `# bash completion for tunwarden
# static commands: %s
# static connection modes: %s
# static profile protocols: %s

_tunwarden()
{
    local cur line value
    local -a runtime_lines runtime_values
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"

    compopt +o default 2>/dev/null || true
    compopt +o nospace 2>/dev/null || true

    if ! mapfile -t runtime_lines < <("${COMP_WORDS[0]}" __complete bash "$COMP_CWORD" "${COMP_WORDS[@]}" 2>/dev/null); then
        return 0
    fi

    for line in "${runtime_lines[@]}"; do
        case "$line" in
            :default-files)
                compopt -o default 2>/dev/null || true
                return 0
                ;;
            :no-files)
                compopt +o default 2>/dev/null || true
                continue
                ;;
            :no-space)
                compopt -o nospace 2>/dev/null || true
                continue
                ;;
            "")
                continue
                ;;
        esac
        value="${line%%$'\t'*}"
        runtime_values+=("$value")
    done

    if ((${#runtime_values[@]} > 0)); then
        COMPREPLY=( $(compgen -W "${runtime_values[*]}" -- "$cur") )
    fi
    return 0
}

complete -o default -F _tunwarden tunwarden
`, completionWords(completionTopLevelCommandNames()), completionWords(completionConnectionModeNames()), completionWords(completionProfileProtocolNames()))
}

func printZshCompletion(w io.Writer) {
	fmt.Fprintf(w, `#compdef tunwarden
# zsh completion for tunwarden
# static commands: %s
# static connection modes: %s
# static profile protocols: %s

_tunwarden() {
  local runtime_output line value description plain
  local -a runtime_lines plain_values described_values
  local cursor=$((CURRENT - 1))

  runtime_output="$("${words[1]}" __complete zsh "$cursor" "${words[@]}" 2>/dev/null)" || return 0
  runtime_lines=("${(@f)runtime_output}")

  for line in "${runtime_lines[@]}"; do
    case "$line" in
      :default-files)
        _files
        return
        ;;
      :no-files)
        continue
        ;;
      :no-space)
        continue
        ;;
      "")
        continue
        ;;
    esac

    value="${line%%$'\t'*}"
    if [[ "$line" == *$'\t'* ]]; then
      description="${line#*$'\t'}"
      described_values+=("${value}:${description}")
    else
      plain_values+=("$value")
    fi
  done

  if (( ${#described_values[@]} > 0 )); then
    for plain in "${plain_values[@]}"; do
      described_values+=("$plain")
    done
    _describe -t tunwarden-completions 'tunwarden completion' described_values
    return
  fi

  if (( ${#plain_values[@]} > 0 )); then
    compadd -- "${plain_values[@]}"
  fi
}

_tunwarden "$@"
`, completionWords(completionTopLevelCommandNames()), completionWords(completionConnectionModeNames()), completionWords(completionProfileProtocolNames()))
}

func printFishCompletion(w io.Writer) {
	fmt.Fprintf(w, `# fish completion for tunwarden
# static commands: %s
# static connection modes: %s
# static profile protocols: %s

function __fish_tunwarden_runtime
    set -l words (commandline -opc)
    set -l current (commandline -ct)

    if test (count $words) -eq 0
        set words tunwarden
    else if test -n "$current"
        if test "$words[-1]" != "$current"
            set -a words "$current"
        end
    else
        set -a words ""
    end

    set -l cursor (math (count $words) - 1)
    command $words[1] __complete fish "$cursor" $words 2>/dev/null
end

function __fish_tunwarden_complete
    for line in (__fish_tunwarden_runtime)
        if not string match -q ':*' -- "$line"
            printf '%%s\n' "$line"
        end
    end
end

function __fish_tunwarden_needs_files
    __fish_tunwarden_runtime | string match -q ':default-files'
end

complete -c tunwarden -f -a '(__fish_tunwarden_complete)'
complete -c tunwarden -n '__fish_tunwarden_needs_files' -F
`, completionWords(completionTopLevelCommandNames()), completionWords(completionConnectionModeNames()), completionWords(completionProfileProtocolNames()))
}

func completionWords(values []string) string {
	return strings.Join(values, " ")
}

func zshWords(values []string) string {
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = "'" + value + "'"
	}
	return strings.Join(quoted, " ")
}
