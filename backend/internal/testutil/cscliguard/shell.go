package cscliguard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// shellCommandWord matches cscli in command position: at the start of a command, after a pipe, a list operator, a
// subshell, a negation, a command substitution, or a keyword that begins a command. It may sit behind `timeout N` or
// `run_interruptible`. The name must end after "cscli", so cscli_cfg and cscli_hub never match.
var shellCommandWord = regexp.MustCompile(
	`(?:^|[|&;(!{]|\$\(|\b(?:if|then|else|elif|do|while|until|exec)\b)\s*(?:(?:timeout\s+\S+|run_interruptible)\s+)*cscli(?:\s|$)`,
)

// shellMarker is the trailing comment that accepts one deliberate start without -c. It must carry a reason.
var shellMarker = regexp.MustCompile(`#\s*cscli-no-config:\s*\S`)

// shellLine is one logical line of a shell script: a line joined with the lines it continues, numbered by its first line.
type shellLine struct {
	number int
	text   string
}

// ShellOffenders reads the shell script at path and returns "file:line: text" for every logical line that starts
// cscli in command position without -c. A `version` subcommand is accepted, since it reads no configuration. A line
// with the marker `# cscli-no-config: <reason>` is a deliberate exception. Comments and quoted text are not commands.
// Wrapper calls (cscli_cfg, cscli_hub) pass the flag themselves and never name cscli in command position.
//
// It returns an error when the file cannot be read, so an empty result means the script was scanned and is clean.
func ShellOffenders(path string) ([]string, error) {
	path = filepath.Clean(path)
	src, err := os.ReadFile(path) //nolint:gosec // G304: a shell script chosen by a test
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	name := filepath.Base(path)
	var offenders []string
	for _, line := range shellLogicalLines(string(src)) {
		if shellMarker.MatchString(line.text) || !startsCSCLIWithoutConfig(shellCode(line.text)) {
			continue
		}
		offenders = append(offenders, fmt.Sprintf("%s:%d: %s", name, line.number, line.text))
	}
	return offenders, nil
}

// shellLogicalLines joins each line that ends in a backslash with the line after it, and returns the joined lines.
func shellLogicalLines(src string) []shellLine {
	var lines []shellLine
	var parts []string
	first := 0
	for i, raw := range strings.Split(src, "\n") {
		if len(parts) == 0 {
			first = i + 1
		}
		trimmed := strings.TrimSpace(raw)
		if continued, ok := strings.CutSuffix(trimmed, `\`); ok {
			parts = append(parts, strings.TrimSpace(continued))
			continue
		}
		parts = append(parts, trimmed)
		lines = append(lines, shellLine{number: first, text: strings.TrimSpace(strings.Join(parts, " "))})
		parts = nil
	}
	if len(parts) > 0 {
		lines = append(lines, shellLine{number: first, text: strings.TrimSpace(strings.Join(parts, " "))})
	}
	return lines
}

// shellCode returns the executable part of a logical line. A comment is cut. The body of a quoted string is blanked,
// unless a double-quoted string holds a command substitution, which runs: an echoed message never reads as a command.
func shellCode(line string) string {
	var code strings.Builder
	var body strings.Builder
	var quote rune // the quote character that is open, or 0 outside quotes
	runes := []rune(line)
	prev := ' '
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case quote == 0 && c == '#' && (prev == ' ' || prev == '\t'):
			return code.String()
		case quote == 0 && (c == '\'' || c == '"'):
			quote = c
			body.Reset()
		case quote == '"' && c == '\\' && i+1 < len(runes):
			body.WriteRune(c)
			body.WriteRune(runes[i+1])
			i++
		case quote != 0 && c == quote:
			code.WriteString(blankedQuote(quote, body.String()))
			quote = 0
		case quote != 0:
			body.WriteRune(c)
		default:
			code.WriteRune(c)
		}
		prev = c
	}
	return code.String()
}

// blankedQuote is what a closed quoted string leaves in the code: its quotes only, unless it runs a command.
func blankedQuote(quote rune, body string) string {
	if quote == '"' && strings.Contains(body, "$(") {
		return `"` + body + `"`
	}
	return string(quote) + string(quote)
}

// startsCSCLIWithoutConfig reports whether the code starts cscli without -c, and is not the version probe.
func startsCSCLIWithoutConfig(code string) bool {
	for _, loc := range shellCommandWord.FindAllStringIndex(code, -1) {
		args := strings.Fields(code[loc[1]:])
		if len(args) == 0 || (args[0] != "-c" && args[0] != "version") {
			return true
		}
	}
	return false
}
