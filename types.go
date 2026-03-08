package apishell

import "context"

type FlagType string

const (
	FlagTypeString FlagType = "string"
	FlagTypeInt    FlagType = "int"
	FlagTypeFloat  FlagType = "float"
	FlagTypeBool   FlagType = "bool"
	FlagTypeJSON   FlagType = "json"
)

type Config struct {
	Commands []CommandSpec `json:"commands,omitempty"`
}

type CommandSpec struct {
	Name      string     `json:"name"`
	ShortHelp string     `json:"short_help,omitempty"`
	LongHelp  string     `json:"long_help,omitempty"`
	Examples  []string   `json:"examples,omitempty"`
	Flags     []FlagSpec `json:"flags,omitempty"`
	Mutating  bool       `json:"mutating,omitempty"`
	HTTP      *HTTPSpec  `json:"http,omitempty"`
}

type FlagSpec struct {
	Name        string   `json:"name"`
	Required    bool     `json:"required,omitempty"`
	Repeatable  bool     `json:"repeatable,omitempty"`
	Description string   `json:"description,omitempty"`
	Type        FlagType `json:"type,omitempty"`
}

type HTTPSpec struct {
	Method               string            `json:"method"`
	URL                  string            `json:"url"`
	Headers              map[string]string `json:"headers,omitempty"`
	Query                map[string]string `json:"query,omitempty"`
	JSONBody             any               `json:"json_body,omitempty"`
	TimeoutMS            int               `json:"timeout_ms,omitempty"`
	MaxResponseBodyBytes int64             `json:"max_response_body_bytes,omitempty"`
	ExpectedContentType  string            `json:"expected_content_type,omitempty"`
	ResponseHeaders      []string          `json:"response_headers,omitempty"`
}

type CommandSummary struct {
	Name      string `json:"name"`
	ShortHelp string `json:"short_help,omitempty"`
	Mutating  bool   `json:"mutating,omitempty"`
}

type CommandDescription struct {
	Name      string     `json:"name"`
	ShortHelp string     `json:"short_help,omitempty"`
	LongHelp  string     `json:"long_help,omitempty"`
	Usage     string     `json:"usage,omitempty"`
	Examples  []string   `json:"examples,omitempty"`
	Flags     []FlagSpec `json:"flags,omitempty"`
	Mutating  bool       `json:"mutating,omitempty"`
}

type Result struct {
	OK       bool           `json:"ok"`
	Verb     string         `json:"verb"`
	Command  string         `json:"command,omitempty"`
	Output   any            `json:"output,omitempty"`
	Error    *ResultError   `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ResultError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

type Command interface {
	Summary() CommandSummary
	Description() CommandDescription
	Run(ctx context.Context, args Args) (Result, error)
}
