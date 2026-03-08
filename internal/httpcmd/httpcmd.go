package httpcmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/samirkhoja/agent-api-shell/internal/interpolate"
)

const (
	defaultRequestTimeout             = 5 * time.Minute
	DefaultMaxResponseBodyBytes int64 = 1 << 20
)

type Spec struct {
	Method               string
	URL                  string
	Headers              map[string]string
	Query                map[string]string
	JSONBody             any
	Timeout              time.Duration
	MaxResponseBodyBytes int64
	ExpectedContentType  string
	ResponseHeaders      []string
}

type Response struct {
	OK       bool
	Output   any
	Metadata map[string]any
	Error    *ResponseError
}

type ResponseError struct {
	Code    string
	Message string
	Details any
}

type responseTooLargeError struct {
	Limit int64
}

func (e responseTooLargeError) Error() string {
	return fmt.Sprintf("response body exceeds %d bytes limit", e.Limit)
}

func Execute(ctx context.Context, client *http.Client, spec Spec, resolver func(interpolate.Ref) (any, error), redact func(string) string) (Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if client == nil {
		client = http.DefaultClient
	}
	if redact == nil {
		redact = func(message string) string { return message }
	}

	var cancel context.CancelFunc
	switch {
	case spec.Timeout > 0:
		ctx, cancel = context.WithTimeout(ctx, spec.Timeout)
	case !hasDeadline(ctx):
		ctx, cancel = context.WithTimeout(ctx, defaultRequestTimeout)
	}
	if cancel != nil {
		defer cancel()
	}

	resolvedURL, err := interpolate.ResolveString(spec.URL, resolver)
	if err != nil {
		return Response{OK: false, Error: &ResponseError{Code: "interpolation_error", Message: redact(err.Error())}}, nil
	}
	parsedURL, err := url.Parse(resolvedURL)
	if err != nil {
		return Response{OK: false, Error: &ResponseError{Code: "invalid_url", Message: redact(err.Error())}}, nil
	}
	queryValues := parsedURL.Query()
	for key, value := range spec.Query {
		resolved, err := interpolate.ResolveString(value, resolver)
		if err != nil {
			return Response{OK: false, Error: &ResponseError{Code: "interpolation_error", Message: redact(err.Error())}}, nil
		}
		queryValues.Set(key, resolved)
	}
	parsedURL.RawQuery = queryValues.Encode()

	var bodyReader io.Reader
	if spec.JSONBody != nil {
		resolvedBody, err := interpolate.ResolveValue(spec.JSONBody, resolver)
		if err != nil {
			return Response{OK: false, Error: &ResponseError{Code: "interpolation_error", Message: redact(err.Error())}}, nil
		}
		bodyBytes, err := json.Marshal(resolvedBody)
		if err != nil {
			return Response{OK: false, Error: &ResponseError{Code: "invalid_body", Message: redact(err.Error())}}, nil
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	request, err := http.NewRequestWithContext(ctx, spec.Method, parsedURL.String(), bodyReader)
	if err != nil {
		return Response{OK: false, Error: &ResponseError{Code: "invalid_request", Message: redact(err.Error())}}, nil
	}
	for key, value := range spec.Headers {
		resolved, err := interpolate.ResolveString(value, resolver)
		if err != nil {
			return Response{OK: false, Error: &ResponseError{Code: "interpolation_error", Message: redact(err.Error())}}, nil
		}
		request.Header.Set(key, resolved)
	}
	if spec.JSONBody != nil && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.Do(request)
	if err != nil {
		return Response{OK: false, Error: &ResponseError{Code: "http_request_failed", Message: redact(err.Error())}}, nil
	}
	defer response.Body.Close()

	contentType := response.Header.Get("Content-Type")
	metadata := buildMetadata(response, contentType, spec.ExpectedContentType, spec.ResponseHeaders)

	body, err := readResponseBody(response.Body, spec.MaxResponseBodyBytes)
	if err != nil {
		var tooLarge responseTooLargeError
		if errors.As(err, &tooLarge) {
			return Response{
				OK:       false,
				Metadata: metadata,
				Error: &ResponseError{
					Code:    "response_too_large",
					Message: tooLarge.Error(),
					Details: map[string]any{"limit_bytes": tooLarge.Limit},
				},
			}, nil
		}
		return Response{
			OK:       false,
			Metadata: metadata,
			Error: &ResponseError{
				Code:    "response_read_failed",
				Message: redact(err.Error()),
			},
		}, nil
	}

	decoded, decodeErr := decodeBody(body, contentType, spec.ExpectedContentType)
	if decodeErr != nil {
		return Response{
			OK:       false,
			Output:   string(body),
			Metadata: metadata,
			Error: &ResponseError{
				Code:    "decode_error",
				Message: redact(decodeErr.Error()),
			},
		}, nil
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Response{
			OK:       false,
			Output:   decoded,
			Metadata: metadata,
			Error: &ResponseError{
				Code:    "http_error",
				Message: fmt.Sprintf("http request failed with status %d", response.StatusCode),
				Details: response.Status,
			},
		}, nil
	}

	return Response{OK: true, Output: decoded, Metadata: metadata}, nil
}

func hasDeadline(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Deadline()
	return ok
}

func buildMetadata(response *http.Response, contentType, expectedContentType string, responseHeaders []string) map[string]any {
	metadata := map[string]any{"status_code": response.StatusCode}
	if contentType != "" {
		metadata["content_type"] = contentType
	} else if expectedContentType != "" {
		metadata["content_type"] = expectedContentType
	}
	if len(responseHeaders) == 0 {
		return metadata
	}

	headers := make(map[string]string)
	for _, name := range responseHeaders {
		if value := response.Header.Get(name); value != "" {
			headers[strings.ToLower(name)] = value
		}
	}
	if len(headers) > 0 {
		metadata["headers"] = headers
	}
	return metadata
}

func readResponseBody(body io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = DefaultMaxResponseBodyBytes
	}

	limited := &io.LimitedReader{R: body, N: limit + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, responseTooLargeError{Limit: limit}
	}
	return data, nil
}

func decodeBody(body []byte, contentType, expectedContentType string) (any, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", nil
	}
	if shouldDecodeJSON(contentType, expectedContentType) {
		var decoded any
		if err := json.Unmarshal(trimmed, &decoded); err != nil {
			return nil, fmt.Errorf("failed to decode JSON response: %w", err)
		}
		return decoded, nil
	}
	return string(body), nil
}

func shouldDecodeJSON(contentType, expectedContentType string) bool {
	candidate := strings.ToLower(contentType)
	if candidate == "" {
		candidate = strings.ToLower(expectedContentType)
	}
	return strings.Contains(candidate, "application/json") || strings.Contains(candidate, "+json") || strings.Contains(candidate, "text/json")
}
