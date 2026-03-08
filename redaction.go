package apishell

import (
	"net/url"
	"sort"
	"strings"
)

type secretRedactor struct {
	values map[string]struct{}
}

func newSecretRedactor() *secretRedactor {
	return &secretRedactor{values: map[string]struct{}{}}
}

func (r *secretRedactor) Add(value string) {
	if r == nil || value == "" {
		return
	}
	for _, candidate := range []string{value, url.QueryEscape(value), url.PathEscape(value)} {
		if candidate == "" {
			continue
		}
		r.values[candidate] = struct{}{}
	}
}

func (r *secretRedactor) Redact(message string) string {
	if r == nil || message == "" || len(r.values) == 0 {
		return message
	}

	secrets := make([]string, 0, len(r.values))
	for value := range r.values {
		secrets = append(secrets, value)
	}
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})

	redacted := message
	for _, secret := range secrets {
		redacted = strings.ReplaceAll(redacted, secret, "[REDACTED]")
	}
	return redacted
}
