package main

import (
	"fmt"
	"sort"
	"strings"
)

// subcommands are the words some commands take before their options.
var subcommands = map[string][]string{
	"providers":  {"list", "add", "remove", "check"},
	"settings":   {"list", "get", "set", "reset"},
	"completion": {"bash", "zsh", "fish"},
}

// completionWords lists what may follow a command: its subcommands, then
// its options as --flags, sorted and without repeats.
func completionWords(c *command) []string {
	words := append([]string(nil), subcommands[c.name]...)
	if c.name == "help" {
		for _, other := range commands {
			words = append(words, other.name)
		}
	}
	seen := map[string]bool{}
	var flags []string
	for _, o := range optionsFor(c.options) {
		if !seen[o.flag] {
			seen[o.flag] = true
			flags = append(flags, "--"+o.flag)
		}
	}
	sort.Strings(flags)
	return append(words, flags...)
}

func runCompletion(c *cmdCtx) error {
	if len(c.args) != 1 {
		return usageError("completion needs one shell: bash, zsh or fish")
	}
	switch c.args[0] {
	case "bash":
		fmt.Fprint(stdout, bashCompletion())
	case "zsh":
		fmt.Fprint(stdout, zshCompletion())
	case "fish":
		fmt.Fprint(stdout, fishCompletion())
	default:
		return usageError(fmt.Sprintf("no completion for %q; use bash, zsh or fish", c.args[0]))
	}
	return nil
}

func commandNames() string {
	var names []string
	for _, c := range commands {
		names = append(names, c.name)
	}
	return strings.Join(names, " ")
}

func bashCompletion() string {
	var b strings.Builder
	b.WriteString("# wordle-solver completion for bash; load with: source <(wordle-solver completion bash)\n")
	b.WriteString("_wordle_solver() {\n")
	b.WriteString("  local cur=${COMP_WORDS[COMP_CWORD]} words\n")
	b.WriteString("  if [ \"$COMP_CWORD\" -eq 1 ]; then\n")
	fmt.Fprintf(&b, "    COMPREPLY=($(compgen -W %q -- \"$cur\"))\n", commandNames())
	b.WriteString("    return\n  fi\n  case ${COMP_WORDS[1]} in\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "    %s) words=%q ;;\n", c.name, strings.Join(completionWords(c), " "))
	}
	b.WriteString("  esac\n  COMPREPLY=($(compgen -W \"$words\" -- \"$cur\"))\n}\n")
	b.WriteString("complete -F _wordle_solver wordle-solver wsolve\n")
	return b.String()
}

// single quotes s for zsh and fish, which both accept '\” inside quotes.
func single(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func zshCompletion() string {
	var b strings.Builder
	b.WriteString("#compdef wordle-solver wsolve\n")
	b.WriteString("# wordle-solver completion for zsh; load with: source <(wordle-solver completion zsh)\n")
	b.WriteString("_wordle_solver() {\n  local -a cmds\n  if (( CURRENT == 2 )); then\n    cmds=(\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "      %s\n", single(c.name+":"+c.summary))
	}
	b.WriteString("    )\n    _describe command cmds\n    return\n  fi\n  case $words[2] in\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "    %s) compadd -- %s ;;\n", c.name, strings.Join(completionWords(c), " "))
	}
	b.WriteString("  esac\n}\n")
	b.WriteString("if [ \"$funcstack[1]\" = _wordle_solver ]; then\n  _wordle_solver \"$@\"\nelse\n  compdef _wordle_solver wordle-solver wsolve\nfi\n")
	return b.String()
}

func fishCompletion() string {
	var b strings.Builder
	b.WriteString("# wordle-solver completion for fish; load with: wordle-solver completion fish | source\n")
	b.WriteString("for prog in wordle-solver wsolve\n")
	b.WriteString("    complete -c $prog -f\n")
	for _, c := range commands {
		fmt.Fprintf(&b, "    complete -c $prog -n __fish_use_subcommand -a %s -d %s\n", c.name, single(c.summary))
	}
	for _, c := range commands {
		cond := single("__fish_seen_subcommand_from " + c.name)
		for _, w := range subcommands[c.name] {
			fmt.Fprintf(&b, "    complete -c $prog -n %s -a %s\n", cond, w)
		}
		for _, o := range optionsFor(c.options) {
			arg := ""
			if o.typ != "bool" {
				arg = " -r"
			}
			fmt.Fprintf(&b, "    complete -c $prog -n %s -l %s%s -d %s\n", cond, o.flag, arg, single(o.usage))
		}
	}
	b.WriteString("end\n")
	return b.String()
}
