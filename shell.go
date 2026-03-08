package apishell

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/samirkhoja/agent-api-shell/internal/parser"
)

type Shell struct {
	mu        sync.RWMutex
	envLookup func(string) (string, bool)
	commands  map[string]Command
}

func New(cfg Config, opts ...Option) (*Shell, error) {
	options := defaultOptions()
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	shell := &Shell{
		envLookup: options.envLookup,
		commands:  map[string]Command{},
	}
	for _, spec := range cfg.Commands {
		cmd, err := newHTTPCommand(spec, options.httpClient, options.envLookup)
		if err != nil {
			return nil, err
		}
		if err := shell.Register(cmd); err != nil {
			return nil, err
		}
	}
	return shell, nil
}

func (s *Shell) Register(cmd Command) error {
	if cmd == nil {
		return fmt.Errorf("%w: nil command", ErrInvalidCommand)
	}
	summary := cmd.Summary()
	name := strings.TrimSpace(summary.Name)
	if err := validateCommandName(name); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCommand, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.commands[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateCommand, name)
	}
	s.commands[name] = cmd
	return nil
}

func (s *Shell) Discover(query string) []CommandSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	needle := strings.ToLower(strings.TrimSpace(query))
	matches := make([]CommandSummary, 0, len(s.commands))
	for _, name := range s.sortedCommandNamesLocked() {
		summary := s.commands[name].Summary()
		if needle != "" {
			haystack := strings.ToLower(summary.Name + " " + summary.ShortHelp)
			if !strings.Contains(haystack, needle) {
				continue
			}
		}
		matches = append(matches, summary)
	}
	return matches
}

func (s *Shell) Describe(name string) (CommandDescription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cmd, ok := s.commands[strings.TrimSpace(name)]
	if !ok {
		return CommandDescription{}, fmt.Errorf("%w: %s", ErrCommandNotFound, name)
	}
	return cmd.Description(), nil
}

func (s *Shell) Execute(ctx context.Context, line string) (Result, error) {
	parsed, err := parser.ParseLine(line)
	if err != nil {
		result := failure("parse_error", err.Error(), nil)
		return result, nil
	}

	switch parsed.Verb {
	case parser.VerbDiscover:
		return Result{OK: true, Verb: parser.VerbDiscover, Output: s.Discover(parsed.Query)}, nil
	case parser.VerbDescribe:
		description, err := s.Describe(parsed.Command)
		if err != nil {
			result := failure("command_not_found", fmt.Sprintf("unknown command %q", parsed.Command), nil)
			result.Verb = parser.VerbDescribe
			result.Command = parsed.Command
			return result, nil
		}
		return Result{OK: true, Verb: parser.VerbDescribe, Command: parsed.Command, Output: description}, nil
	case parser.VerbRun:
		s.mu.RLock()
		cmd, ok := s.commands[parsed.Command]
		s.mu.RUnlock()
		if !ok {
			result := failure("command_not_found", fmt.Sprintf("unknown command %q", parsed.Command), nil)
			result.Verb = parser.VerbRun
			result.Command = parsed.Command
			return result, nil
		}
		result, err := cmd.Run(ctx, NewArgs(parsed.Flags))
		if err != nil {
			wrapped := failure("execution_error", err.Error(), nil)
			wrapped.Verb = parser.VerbRun
			wrapped.Command = parsed.Command
			return wrapped, nil
		}
		if result.Verb == "" {
			result.Verb = parser.VerbRun
		}
		if result.Command == "" {
			result.Command = parsed.Command
		}
		return result, nil
	default:
		result := failure("unsupported_verb", fmt.Sprintf("unsupported verb %q", parsed.Verb), nil)
		result.Verb = parsed.Verb
		return result, nil
	}
}

func (s *Shell) sortedCommandNamesLocked() []string {
	names := make([]string, 0, len(s.commands))
	for name := range s.commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
