package interpolate

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Ref struct {
	Kind string
	Name string
}

type segment struct {
	start int
	end   int
	ref   Ref
}

func ValidateString(input string, flags map[string]struct{}) error {
	segments, err := parseSegments(input)
	if err != nil {
		return err
	}
	for _, segment := range segments {
		switch segment.ref.Kind {
		case "flag":
			if _, ok := flags[segment.ref.Name]; !ok {
				return fmt.Errorf("unknown flag %q", segment.ref.Name)
			}
		case "env":
			if segment.ref.Name == "" {
				return fmt.Errorf("invalid environment reference")
			}
		default:
			return fmt.Errorf("unsupported reference kind %q", segment.ref.Kind)
		}
	}
	return nil
}

func ValidateValue(value any, flags map[string]struct{}) error {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return ValidateString(typed, flags)
	case []any:
		for _, item := range typed {
			if err := ValidateValue(item, flags); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		for _, item := range typed {
			if err := ValidateValue(item, flags); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func ResolveString(input string, resolver func(Ref) (any, error)) (string, error) {
	segments, err := parseSegments(input)
	if err != nil {
		return "", err
	}
	if len(segments) == 0 {
		return input, nil
	}

	var builder strings.Builder
	cursor := 0
	for _, segment := range segments {
		builder.WriteString(input[cursor:segment.start])
		value, err := resolver(segment.ref)
		if err != nil {
			return "", err
		}
		stringValue, err := stringify(value)
		if err != nil {
			return "", err
		}
		builder.WriteString(stringValue)
		cursor = segment.end
	}
	builder.WriteString(input[cursor:])
	return builder.String(), nil
}

func ResolveValue(value any, resolver func(Ref) (any, error)) (any, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case string:
		segments, err := parseSegments(typed)
		if err != nil {
			return nil, err
		}
		if len(segments) == 1 && segments[0].start == 0 && segments[0].end == len(typed) {
			return resolver(segments[0].ref)
		}
		return ResolveString(typed, resolver)
	case []any:
		resolved := make([]any, len(typed))
		for i, item := range typed {
			value, err := ResolveValue(item, resolver)
			if err != nil {
				return nil, err
			}
			resolved[i] = value
		}
		return resolved, nil
	case map[string]any:
		resolved := make(map[string]any, len(typed))
		for key, item := range typed {
			value, err := ResolveValue(item, resolver)
			if err != nil {
				return nil, err
			}
			resolved[key] = value
		}
		return resolved, nil
	default:
		return typed, nil
	}
}

func parseSegments(input string) ([]segment, error) {
	segments := make([]segment, 0)
	cursor := 0
	for cursor < len(input) {
		start := strings.Index(input[cursor:], "${")
		if start < 0 {
			break
		}
		start += cursor
		end := strings.IndexByte(input[start+2:], '}')
		if end < 0 {
			return nil, fmt.Errorf("unterminated interpolation")
		}
		end += start + 2
		ref, err := parseRef(input[start+2 : end])
		if err != nil {
			return nil, err
		}
		segments = append(segments, segment{start: start, end: end + 1, ref: ref})
		cursor = end + 1
	}
	return segments, nil
}

func parseRef(raw string) (Ref, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Ref{}, fmt.Errorf("invalid interpolation %q", raw)
	}
	switch parts[0] {
	case "flag", "env":
		return Ref{Kind: parts[0], Name: parts[1]}, nil
	default:
		return Ref{}, fmt.Errorf("unsupported interpolation kind %q", parts[0])
	}
}

func stringify(value any) (string, error) {
	switch typed := value.(type) {
	case nil:
		return "", nil
	case string:
		return typed, nil
	case fmt.Stringer:
		return typed.String(), nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	default:
		bytes, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		if len(bytes) >= 2 && bytes[0] == '"' && bytes[len(bytes)-1] == '"' {
			return string(bytes[1 : len(bytes)-1]), nil
		}
		return string(bytes), nil
	}
}
