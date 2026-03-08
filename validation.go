package apishell

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/samirkhoja/agent-api-shell/internal/interpolate"
)

var (
	commandNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	flagNamePattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
)

func validateCommandName(name string) error {
	if name == "" {
		return fmt.Errorf("command name is required")
	}
	if !commandNamePattern.MatchString(name) {
		return fmt.Errorf("invalid command name %q", name)
	}
	return nil
}

func validateFlagName(name string) error {
	if name == "" {
		return fmt.Errorf("flag name is required")
	}
	if !flagNamePattern.MatchString(name) {
		return fmt.Errorf("invalid flag name %q", name)
	}
	return nil
}

func normalizeFlagType(flagType FlagType) (FlagType, error) {
	switch FlagType(strings.TrimSpace(string(flagType))) {
	case "", FlagTypeString:
		return FlagTypeString, nil
	case FlagTypeInt:
		return FlagTypeInt, nil
	case FlagTypeFloat:
		return FlagTypeFloat, nil
	case FlagTypeBool:
		return FlagTypeBool, nil
	case FlagTypeJSON:
		return FlagTypeJSON, nil
	default:
		return "", fmt.Errorf("unsupported flag type %q", flagType)
	}
}

func normalizeFlagSpec(flag FlagSpec) (FlagSpec, error) {
	normalized := flag
	normalized.Name = strings.TrimSpace(flag.Name)
	normalized.Description = strings.TrimSpace(flag.Description)
	if err := validateFlagName(normalized.Name); err != nil {
		return FlagSpec{}, err
	}
	flagType, err := normalizeFlagType(flag.Type)
	if err != nil {
		return FlagSpec{}, err
	}
	normalized.Type = flagType
	return normalized, nil
}

func validateCommandSpec(spec CommandSpec) (CommandSpec, error) {
	normalized := spec
	normalized.Name = strings.TrimSpace(spec.Name)
	normalized.ShortHelp = strings.TrimSpace(spec.ShortHelp)
	normalized.LongHelp = strings.TrimSpace(spec.LongHelp)
	if err := validateCommandName(normalized.Name); err != nil {
		return CommandSpec{}, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	if spec.HTTP == nil {
		return CommandSpec{}, fmt.Errorf("%w: command %q requires an http spec", ErrInvalidConfig, normalized.Name)
	}

	normalized.Examples = trimAndCloneStrings(spec.Examples)
	normalized.Flags = make([]FlagSpec, 0, len(spec.Flags))
	seenFlags := make(map[string]struct{}, len(spec.Flags))
	flagSet := make(map[string]struct{}, len(spec.Flags))
	for _, flag := range spec.Flags {
		normalizedFlag, err := normalizeFlagSpec(flag)
		if err != nil {
			return CommandSpec{}, fmt.Errorf("%w: command %q: %v", ErrInvalidConfig, normalized.Name, err)
		}
		if _, exists := seenFlags[normalizedFlag.Name]; exists {
			return CommandSpec{}, fmt.Errorf("%w: command %q: duplicate flag %q", ErrInvalidConfig, normalized.Name, normalizedFlag.Name)
		}
		seenFlags[normalizedFlag.Name] = struct{}{}
		flagSet[normalizedFlag.Name] = struct{}{}
		normalized.Flags = append(normalized.Flags, normalizedFlag)
	}

	httpSpec := *spec.HTTP
	httpSpec.Method = strings.ToUpper(strings.TrimSpace(httpSpec.Method))
	httpSpec.URL = strings.TrimSpace(httpSpec.URL)
	httpSpec.Headers = cloneStringMap(httpSpec.Headers)
	httpSpec.Query = cloneStringMap(httpSpec.Query)
	httpSpec.ExpectedContentType = strings.TrimSpace(httpSpec.ExpectedContentType)
	httpSpec.ResponseHeaders = trimAndCloneStrings(httpSpec.ResponseHeaders)
	if httpSpec.Method == "" {
		return CommandSpec{}, fmt.Errorf("%w: command %q: http method is required", ErrInvalidConfig, normalized.Name)
	}
	if httpSpec.URL == "" {
		return CommandSpec{}, fmt.Errorf("%w: command %q: http url is required", ErrInvalidConfig, normalized.Name)
	}
	if httpSpec.TimeoutMS < 0 {
		return CommandSpec{}, fmt.Errorf("%w: command %q: timeout_ms must be >= 0", ErrInvalidConfig, normalized.Name)
	}
	if httpSpec.MaxResponseBodyBytes < 0 {
		return CommandSpec{}, fmt.Errorf("%w: command %q: max_response_body_bytes must be >= 0", ErrInvalidConfig, normalized.Name)
	}
	if err := interpolate.ValidateString(httpSpec.URL, flagSet); err != nil {
		return CommandSpec{}, fmt.Errorf("%w: command %q: url: %v", ErrInvalidConfig, normalized.Name, err)
	}
	for key, value := range httpSpec.Headers {
		if strings.TrimSpace(key) == "" {
			return CommandSpec{}, fmt.Errorf("%w: command %q: header name is required", ErrInvalidConfig, normalized.Name)
		}
		if err := interpolate.ValidateString(value, flagSet); err != nil {
			return CommandSpec{}, fmt.Errorf("%w: command %q: header %q: %v", ErrInvalidConfig, normalized.Name, key, err)
		}
	}
	for key, value := range httpSpec.Query {
		if strings.TrimSpace(key) == "" {
			return CommandSpec{}, fmt.Errorf("%w: command %q: query key is required", ErrInvalidConfig, normalized.Name)
		}
		if err := interpolate.ValidateString(value, flagSet); err != nil {
			return CommandSpec{}, fmt.Errorf("%w: command %q: query %q: %v", ErrInvalidConfig, normalized.Name, key, err)
		}
	}
	if err := interpolate.ValidateValue(httpSpec.JSONBody, flagSet); err != nil {
		return CommandSpec{}, fmt.Errorf("%w: command %q: json_body: %v", ErrInvalidConfig, normalized.Name, err)
	}

	normalized.HTTP = &httpSpec
	return normalized, nil
}

func trimAndCloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		cloned = append(cloned, trimmed)
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func buildUsage(spec CommandSpec) string {
	parts := []string{"run", spec.Name}
	for _, flag := range spec.Flags {
		token := fmt.Sprintf("--%s <%s>", flag.Name, flag.Type)
		if flag.Repeatable {
			token += "..."
		}
		if flag.Required {
			parts = append(parts, token)
			continue
		}
		parts = append(parts, "["+token+"]")
	}
	return strings.Join(parts, " ")
}
