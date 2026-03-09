package apishell

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	internalhttpcmd "github.com/samirkhoja/agent-api-shell/internal/httpcmd"
)

type echoCommand struct{}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func (echoCommand) Summary() CommandSummary {
	return CommandSummary{Name: "echo", ShortHelp: "Echo a message"}
}

func (echoCommand) Description() CommandDescription {
	return CommandDescription{
		Name:      "echo",
		ShortHelp: "Echo a message",
		Usage:     "run echo --message <string>",
		Flags: []FlagSpec{{
			Name:        "message",
			Required:    true,
			Description: "Message to echo",
			Type:        FlagTypeString,
		}},
	}
}

func (echoCommand) Run(ctx context.Context, args Args) (Result, error) {
	message, ok := args.Value("message")
	if !ok {
		return Result{OK: false, Error: &ResultError{Code: "missing_flag", Message: "missing required flag \"message\""}}, nil
	}
	return Result{OK: true, Verb: "run", Command: "echo", Output: map[string]any{"message": message}}, nil
}

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"commands":[{"name":"weather","short_help":"Fetch weather","http":{"method":"GET","url":"https://example.com/weather"}}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if len(cfg.Commands) != 1 || cfg.Commands[0].Name != "weather" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestShellRegisterDiscoverDescribeAndExecuteCustomCommand(t *testing.T) {
	shell, err := New(Config{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := shell.Register(echoCommand{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	summaries := shell.Discover("echo")
	if len(summaries) != 1 || summaries[0].Name != "echo" {
		t.Fatalf("unexpected discover summaries: %#v", summaries)
	}

	description, err := shell.Describe("echo")
	if err != nil {
		t.Fatalf("Describe returned error: %v", err)
	}
	if description.Usage != "run echo --message <string>" {
		t.Fatalf("unexpected description: %#v", description)
	}

	result, err := shell.Execute(context.Background(), `run echo --message "hello"`)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.OK || result.Command != "echo" {
		t.Fatalf("unexpected result: %#v", result)
	}
	output, ok := result.Output.(map[string]any)
	if !ok || output["message"] != "hello" {
		t.Fatalf("unexpected output: %#v", result.Output)
	}
}

func TestShellExecuteListReturnsAllCommands(t *testing.T) {
	shell, err := New(Config{Commands: []CommandSpec{{
		Name:      "weather",
		ShortHelp: "Fetch weather",
		HTTP:      &HTTPSpec{Method: http.MethodGet, URL: "https://example.com/weather"},
	}}})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := shell.Register(echoCommand{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	result, err := shell.Execute(context.Background(), `list`)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.OK || result.Verb != "list" {
		t.Fatalf("unexpected result: %#v", result)
	}

	summaries, ok := result.Output.([]CommandSummary)
	if !ok {
		t.Fatalf("unexpected output: %#v", result.Output)
	}
	want := []CommandSummary{
		{Name: "echo", ShortHelp: "Echo a message"},
		{Name: "weather", ShortHelp: "Fetch weather"},
	}
	if !reflect.DeepEqual(summaries, want) {
		t.Fatalf("unexpected output: %#v", summaries)
	}
}

func TestNewRejectsDuplicateCommands(t *testing.T) {
	_, err := New(Config{Commands: []CommandSpec{
		{Name: "weather", ShortHelp: "one", HTTP: &HTTPSpec{Method: http.MethodGet, URL: "https://example.com/one"}},
		{Name: "weather", ShortHelp: "two", HTTP: &HTTPSpec{Method: http.MethodGet, URL: "https://example.com/two"}},
	}})
	if !errors.Is(err, ErrDuplicateCommand) {
		t.Fatalf("expected duplicate command error, got %v", err)
	}
}

func TestNewRejectsUnknownInterpolationFlag(t *testing.T) {
	_, err := New(Config{Commands: []CommandSpec{{
		Name:      "weather",
		ShortHelp: "Fetch weather",
		Flags:     []FlagSpec{{Name: "city", Type: FlagTypeString}},
		HTTP: &HTTPSpec{
			Method: http.MethodGet,
			URL:    "https://example.com/weather",
			Query:  map[string]string{"q": "${flag.missing}"},
		},
	}}})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected invalid config error, got %v", err)
	}
}

func TestNewRejectsNegativeMaxResponseBodyBytes(t *testing.T) {
	_, err := New(Config{Commands: []CommandSpec{{
		Name:      "weather",
		ShortHelp: "Fetch weather",
		HTTP: &HTTPSpec{
			Method:               http.MethodGet,
			URL:                  "https://example.com/weather",
			MaxResponseBodyBytes: -1,
		},
	}}})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected invalid config error, got %v", err)
	}
}

func TestHTTPCommandGETJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if got := r.URL.Query().Get("city"); got != "San Francisco" {
			t.Fatalf("unexpected city query: %q", got)
		}
		if got := r.URL.Query().Get("units"); got != "metric" {
			t.Fatalf("unexpected units query: %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", "req-123")
		_, _ = w.Write([]byte(`{"city":"San Francisco","temp_c":18}`))
	}))
	defer server.Close()

	shell, err := New(
		Config{Commands: []CommandSpec{{
			Name:      "weather",
			ShortHelp: "Fetch weather",
			Flags: []FlagSpec{
				{Name: "city", Required: true, Type: FlagTypeString},
				{Name: "units", Type: FlagTypeString},
			},
			HTTP: &HTTPSpec{
				Method:          http.MethodGet,
				URL:             server.URL + "/weather",
				Headers:         map[string]string{"Authorization": "Bearer ${env.API_TOKEN}"},
				Query:           map[string]string{"city": "${flag.city}", "units": "${flag.units}"},
				ResponseHeaders: []string{"X-Request-ID"},
			},
		}}},
		WithEnvLookup(func(name string) (string, bool) {
			if name == "API_TOKEN" {
				return "secret-token", true
			}
			return "", false
		}),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	result, err := shell.Execute(context.Background(), `run weather --city "San Francisco" --units metric`)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("unexpected result error: %#v", result)
	}
	output, ok := result.Output.(map[string]any)
	if !ok || output["city"] != "San Francisco" {
		t.Fatalf("unexpected output: %#v", result.Output)
	}
	if got := result.Metadata["status_code"]; got != 200 {
		t.Fatalf("unexpected status code metadata: %#v", result.Metadata)
	}
	headers, ok := result.Metadata["headers"].(map[string]string)
	if !ok || headers["x-request-id"] != "req-123" {
		t.Fatalf("unexpected response headers: %#v", result.Metadata["headers"])
	}
}

func TestHTTPCommandPOSTJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("Decode returned error: %v", err)
		}
		if body["name"] != "demo" {
			t.Fatalf("unexpected name: %#v", body)
		}
		payload, ok := body["payload"].(map[string]any)
		if !ok || payload["kind"] != "example" {
			t.Fatalf("unexpected payload: %#v", body)
		}
		if body["token"] != "secret-token" {
			t.Fatalf("unexpected token: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"created":true}`))
	}))
	defer server.Close()

	shell, err := New(
		Config{Commands: []CommandSpec{{
			Name:      "create-item",
			ShortHelp: "Create an item",
			Flags: []FlagSpec{
				{Name: "name", Required: true, Type: FlagTypeString},
				{Name: "payload", Required: true, Type: FlagTypeJSON},
			},
			HTTP: &HTTPSpec{
				Method: http.MethodPost,
				URL:    server.URL + "/items",
				JSONBody: map[string]any{
					"name":    "${flag.name}",
					"payload": "${flag.payload}",
					"token":   "${env.API_TOKEN}",
				},
			},
		}}},
		WithEnvLookup(func(name string) (string, bool) {
			if name == "API_TOKEN" {
				return "secret-token", true
			}
			return "", false
		}),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	result, err := shell.Execute(context.Background(), `run create-item --name demo --payload '{"kind":"example"}'`)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("unexpected result: %#v", result)
	}
	if got := result.Metadata["status_code"]; got != 201 {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestHTTPCommandAppliesDefaultDeadlineWhenUnset(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			deadline, ok := request.Context().Deadline()
			if !ok {
				return nil, fmt.Errorf("request context missing deadline")
			}
			remaining := time.Until(deadline)
			if remaining < 4*time.Minute || remaining > 6*time.Minute {
				return nil, fmt.Errorf("unexpected deadline window: %s", remaining)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		}),
	}

	shell, err := New(
		Config{Commands: []CommandSpec{{
			Name:      "ping",
			ShortHelp: "Ping a service",
			HTTP:      &HTTPSpec{Method: http.MethodGet, URL: "https://example.com/ping"},
		}}},
		WithHTTPClient(client),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	result, err := shell.Execute(context.Background(), `run ping`)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !result.OK {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestHTTPCommandRejectsOversizedResponseBody(t *testing.T) {
	payload := strings.Repeat("a", int(internalhttpcmd.DefaultMaxResponseBodyBytes)+1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(payload))
	}))
	defer server.Close()

	shell, err := New(Config{Commands: []CommandSpec{{
		Name:      "large-response",
		ShortHelp: "Return too much data",
		HTTP: &HTTPSpec{
			Method: http.MethodGet,
			URL:    server.URL + "/large",
		},
	}}})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	result, err := shell.Execute(context.Background(), `run large-response`)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.OK || result.Error == nil || result.Error.Code != "response_too_large" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if got := result.Metadata["status_code"]; got != 200 {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestHTTPCommandMissingEnvReturnsStructuredError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	shell, err := New(
		Config{Commands: []CommandSpec{{
			Name:      "secure",
			ShortHelp: "Secure command",
			Flags:     []FlagSpec{{Name: "id", Required: true, Type: FlagTypeString}},
			HTTP: &HTTPSpec{
				Method:  http.MethodGet,
				URL:     server.URL + "/secure",
				Headers: map[string]string{"Authorization": "Bearer ${env.API_TOKEN}"},
			},
		}}},
		WithEnvLookup(func(string) (string, bool) { return "", false }),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	result, err := shell.Execute(context.Background(), `run secure --id 123`)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.OK || result.Error == nil || result.Error.Code != "interpolation_error" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !strings.Contains(result.Error.Message, "API_TOKEN") {
		t.Fatalf("unexpected error message: %#v", result.Error)
	}
}

func TestHTTPCommandRedactsSecretsInTransportErrors(t *testing.T) {
	secret := "tok en/123"
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("upstream rejected %s", request.URL.String())
		}),
	}

	shell, err := New(
		Config{Commands: []CommandSpec{{
			Name:      "secure",
			ShortHelp: "Secure command",
			Flags:     []FlagSpec{{Name: "id", Required: true, Type: FlagTypeString}},
			HTTP: &HTTPSpec{
				Method: http.MethodGet,
				URL:    "https://example.com/secure?token=${env.API_TOKEN}&id=${flag.id}",
			},
		}}},
		WithHTTPClient(client),
		WithEnvLookup(func(name string) (string, bool) {
			if name == "API_TOKEN" {
				return secret, true
			}
			return "", false
		}),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	result, err := shell.Execute(context.Background(), `run secure --id 123`)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.OK || result.Error == nil || result.Error.Code != "http_request_failed" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if strings.Contains(result.Error.Message, secret) {
		t.Fatalf("secret leaked in error message: %#v", result.Error)
	}
	if strings.Contains(result.Error.Message, url.QueryEscape(secret)) {
		t.Fatalf("escaped secret leaked in error message: %#v", result.Error)
	}
	if !strings.Contains(result.Error.Message, "[REDACTED]") {
		t.Fatalf("expected redacted placeholder in error message: %#v", result.Error)
	}
}

func TestHTTPCommandNon2xxReturnsStructuredError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	shell, err := New(Config{Commands: []CommandSpec{{
		Name:      "reject",
		ShortHelp: "Reject request",
		HTTP: &HTTPSpec{
			Method: http.MethodGet,
			URL:    server.URL + "/reject",
		},
	}}})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	result, err := shell.Execute(context.Background(), `run reject`)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.OK || result.Error == nil || result.Error.Code != "http_error" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if got := result.Metadata["status_code"]; got != 400 {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
}

func TestDiscoverStableOrderAcrossConfigAndRegister(t *testing.T) {
	shell, err := New(Config{Commands: []CommandSpec{{
		Name:      "zeta",
		ShortHelp: "last",
		HTTP:      &HTTPSpec{Method: http.MethodGet, URL: "https://example.com/zeta"},
	}}})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := shell.Register(echoCommand{}); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}

	got := shell.Discover("")
	want := []CommandSummary{{Name: "echo", ShortHelp: "Echo a message"}, {Name: "zeta", ShortHelp: "last"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected discover output: %#v", got)
	}
}
