package apishell

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/samirkhoja/agent-api-shell/internal/httpcmd"
	"github.com/samirkhoja/agent-api-shell/internal/interpolate"
)

type httpCommand struct {
	spec      CommandSpec
	client    *http.Client
	envLookup func(string) (string, bool)
	flags     map[string]FlagSpec
}

func newHTTPCommand(spec CommandSpec, client *http.Client, envLookup func(string) (string, bool)) (*httpCommand, error) {
	normalized, err := validateCommandSpec(spec)
	if err != nil {
		return nil, err
	}
	flagMap := make(map[string]FlagSpec, len(normalized.Flags))
	for _, flag := range normalized.Flags {
		flagMap[flag.Name] = flag
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &httpCommand{
		spec:      normalized,
		client:    client,
		envLookup: envLookup,
		flags:     flagMap,
	}, nil
}

func (c *httpCommand) Summary() CommandSummary {
	return CommandSummary{
		Name:      c.spec.Name,
		ShortHelp: c.spec.ShortHelp,
		Mutating:  c.spec.Mutating,
	}
}

func (c *httpCommand) Description() CommandDescription {
	return CommandDescription{
		Name:      c.spec.Name,
		ShortHelp: c.spec.ShortHelp,
		LongHelp:  c.spec.LongHelp,
		Usage:     buildUsage(c.spec),
		Examples:  cloneStringSlice(c.spec.Examples),
		Flags:     append([]FlagSpec(nil), c.spec.Flags...),
		Mutating:  c.spec.Mutating,
	}
}

func (c *httpCommand) Run(ctx context.Context, args Args) (Result, error) {
	parsedArgs, resultErr := c.parseArgs(args)
	if resultErr != nil {
		return Result{
			OK:      false,
			Verb:    "run",
			Command: c.spec.Name,
			Error:   resultErr,
		}, nil
	}

	redactor := newSecretRedactor()
	response, err := httpcmd.Execute(ctx, c.client, httpcmd.Spec{
		Method:               c.spec.HTTP.Method,
		URL:                  c.spec.HTTP.URL,
		Headers:              cloneStringMap(c.spec.HTTP.Headers),
		Query:                cloneStringMap(c.spec.HTTP.Query),
		JSONBody:             c.spec.HTTP.JSONBody,
		Timeout:              time.Duration(c.spec.HTTP.TimeoutMS) * time.Millisecond,
		MaxResponseBodyBytes: c.spec.HTTP.MaxResponseBodyBytes,
		ExpectedContentType:  c.spec.HTTP.ExpectedContentType,
		ResponseHeaders:      cloneStringSlice(c.spec.HTTP.ResponseHeaders),
	}, func(ref interpolate.Ref) (any, error) {
		switch ref.Kind {
		case "flag":
			value, ok := parsedArgs[ref.Name]
			if !ok {
				return nil, fmt.Errorf("missing flag %q", ref.Name)
			}
			return value, nil
		case "env":
			if c.envLookup == nil {
				return nil, fmt.Errorf("missing environment variable %q", ref.Name)
			}
			value, ok := c.envLookup(ref.Name)
			if !ok {
				return nil, fmt.Errorf("missing environment variable %q", ref.Name)
			}
			redactor.Add(value)
			return value, nil
		default:
			return nil, fmt.Errorf("unsupported reference kind %q", ref.Kind)
		}
	}, redactor.Redact)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		OK:       response.OK,
		Verb:     "run",
		Command:  c.spec.Name,
		Output:   response.Output,
		Metadata: cloneResultMetadata(response.Metadata),
	}
	if response.Error != nil {
		result.Error = &ResultError{
			Code:    response.Error.Code,
			Message: response.Error.Message,
			Details: response.Error.Details,
		}
	}
	return result, nil
}

func (c *httpCommand) parseArgs(args Args) (map[string]any, *ResultError) {
	raw := args.Map()
	for name := range raw {
		if _, ok := c.flags[name]; !ok {
			return nil, &ResultError{
				Code:    "unknown_flag",
				Message: fmt.Sprintf("unknown flag %q", name),
			}
		}
	}

	parsed := make(map[string]any, len(c.flags))
	for _, flag := range c.spec.Flags {
		values := raw[flag.Name]
		if len(values) == 0 {
			if flag.Required {
				return nil, &ResultError{
					Code:    "missing_flag",
					Message: fmt.Sprintf("missing required flag %q", flag.Name),
				}
			}
			continue
		}
		if !flag.Repeatable && len(values) > 1 {
			return nil, &ResultError{
				Code:    "repeated_flag",
				Message: fmt.Sprintf("flag %q does not accept repeated values", flag.Name),
			}
		}

		converted := make([]any, 0, len(values))
		for _, value := range values {
			parsedValue, err := coerceFlagValue(flag.Type, value)
			if err != nil {
				return nil, &ResultError{
					Code:    "invalid_flag_value",
					Message: fmt.Sprintf("invalid value for flag %q", flag.Name),
					Details: err.Error(),
				}
			}
			converted = append(converted, parsedValue)
		}
		if flag.Repeatable {
			parsed[flag.Name] = converted
			continue
		}
		parsed[flag.Name] = converted[0]
	}
	return parsed, nil
}

func coerceFlagValue(flagType FlagType, value string) (any, error) {
	switch flagType {
	case FlagTypeString:
		return value, nil
	case FlagTypeInt:
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case FlagTypeFloat:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case FlagTypeBool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		return parsed, nil
	case FlagTypeJSON:
		var parsed any
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("unsupported flag type %q", flagType)
	}
}
