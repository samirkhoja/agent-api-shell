package apishell

import (
	"fmt"
	"net/http"
	"os"
)

type Option func(*shellOptions) error

type shellOptions struct {
	httpClient *http.Client
	envLookup  func(string) (string, bool)
}

func defaultOptions() shellOptions {
	return shellOptions{
		httpClient: http.DefaultClient,
		envLookup:  os.LookupEnv,
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(opts *shellOptions) error {
		if client == nil {
			return fmt.Errorf("%w: nil HTTP client", ErrInvalidConfig)
		}
		opts.httpClient = client
		return nil
	}
}

func WithEnvLookup(lookup func(string) (string, bool)) Option {
	return func(opts *shellOptions) error {
		if lookup == nil {
			return fmt.Errorf("%w: nil environment lookup", ErrInvalidConfig)
		}
		opts.envLookup = lookup
		return nil
	}
}
