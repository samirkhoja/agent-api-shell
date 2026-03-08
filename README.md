# API Shell

`agent-api-shell` is a standalone Go library that exposes a minimal pseudo shell for LLM-driven tool discovery and execution.

HTTP-backed commands default to a 5 minute request deadline and a 1 MiB response body limit. You can override these per command with `timeout_ms` and `max_response_body_bytes`.

## Capabilities

- `discover [query]`
- `describe <command>`
- `run <command> --flag value ...`

Config-defined commands are HTTP/API-backed in v1. The library also supports programmatic command registration through the `Command` interface.

## Install

```bash
go get github.com/samirkhoja/agent-api-shell
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    apishell "github.com/samirkhoja/agent-api-shell"
)

func main() {
    shell, err := apishell.New(apishell.Config{})
    if err != nil {
        log.Fatal(err)
    }

    result, err := shell.Execute(context.Background(), `discover weather`)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.OK)
}
```

## Config Example

```json
{
  "commands": [
    {
      "name": "weather",
      "short_help": "Fetch current weather for a city",
      "long_help": "Calls a weather API and returns the normalized response body.",
      "examples": [
        "run weather --city \"San Francisco\" --units metric"
      ],
      "flags": [
        {
          "name": "city",
          "required": true,
          "description": "City name",
          "type": "string"
        },
        {
          "name": "units",
          "description": "Units system",
          "type": "string"
        }
      ],
      "http": {
        "method": "GET",
        "url": "https://api.example.com/weather",
        "timeout_ms": 10000,
        "max_response_body_bytes": 1048576,
        "headers": {
          "Authorization": "Bearer ${env.WEATHER_API_KEY}"
        },
        "query": {
          "city": "${flag.city}",
          "units": "${flag.units}"
        },
        "expected_content_type": "application/json",
        "response_headers": ["X-Request-ID"]
      }
    }
  ]
}
```
