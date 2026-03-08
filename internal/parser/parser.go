package parser

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	VerbDiscover = "discover"
	VerbDescribe = "describe"
	VerbRun      = "run"
)

type ParsedCommand struct {
	Verb    string
	Query   string
	Command string
	Flags   map[string][]string
}

func ParseLine(line string) (ParsedCommand, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return ParsedCommand{}, fmt.Errorf("command line is required")
	}
	if strings.ContainsAny(line, "\r\n") {
		return ParsedCommand{}, fmt.Errorf("multi-line input is not supported")
	}

	tokens, err := tokenize(line)
	if err != nil {
		return ParsedCommand{}, err
	}
	if len(tokens) == 0 {
		return ParsedCommand{}, fmt.Errorf("command line is required")
	}

	parsed := ParsedCommand{Verb: tokens[0], Flags: map[string][]string{}}
	switch tokens[0] {
	case VerbDiscover:
		parsed.Query = strings.Join(tokens[1:], " ")
		return parsed, nil
	case VerbDescribe:
		if len(tokens) != 2 {
			return ParsedCommand{}, fmt.Errorf("describe requires exactly one command name")
		}
		parsed.Command = tokens[1]
		return parsed, nil
	case VerbRun:
		if len(tokens) < 2 {
			return ParsedCommand{}, fmt.Errorf("run requires a command name")
		}
		parsed.Command = tokens[1]
		for i := 2; i < len(tokens); i++ {
			token := tokens[i]
			if !strings.HasPrefix(token, "--") || len(token) <= 2 {
				return ParsedCommand{}, fmt.Errorf("unexpected token %q", token)
			}
			flagToken := token[2:]
			if flagToken == "" {
				return ParsedCommand{}, fmt.Errorf("flag name is required")
			}

			name := flagToken
			value := ""
			if idx := strings.Index(flagToken, "="); idx >= 0 {
				name = flagToken[:idx]
				value = flagToken[idx+1:]
			} else {
				if i+1 >= len(tokens) {
					return ParsedCommand{}, fmt.Errorf("flag %q requires a value", name)
				}
				next := tokens[i+1]
				if strings.HasPrefix(next, "--") {
					return ParsedCommand{}, fmt.Errorf("flag %q requires a value", name)
				}
				value = next
				i++
			}
			if name == "" {
				return ParsedCommand{}, fmt.Errorf("flag name is required")
			}
			parsed.Flags[name] = append(parsed.Flags[name], value)
		}
		return parsed, nil
	default:
		return ParsedCommand{}, fmt.Errorf("unsupported verb %q", tokens[0])
	}
}

func tokenize(line string) ([]string, error) {
	var (
		tokens       []string
		current      strings.Builder
		quote        rune
		escaped      bool
		tokenStarted bool
	)

	flush := func() {
		if !tokenStarted {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
		tokenStarted = false
	}

	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			tokenStarted = true
			escaped = false
			continue
		}

		switch quote {
		case 0:
			switch {
			case r == '\\':
				escaped = true
				tokenStarted = true
			case r == '\'' || r == '"':
				quote = r
				tokenStarted = true
			case unicode.IsSpace(r):
				flush()
			default:
				current.WriteRune(r)
				tokenStarted = true
			}
		case '\'':
			if r == '\'' {
				quote = 0
				continue
			}
			current.WriteRune(r)
			tokenStarted = true
		case '"':
			switch r {
			case '"':
				quote = 0
			case '\\':
				escaped = true
				tokenStarted = true
			default:
				current.WriteRune(r)
				tokenStarted = true
			}
		}
	}
	if escaped {
		return nil, fmt.Errorf("unterminated escape sequence")
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	flush()
	return tokens, nil
}
