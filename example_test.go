package apishell

import (
	"context"
	"fmt"
)

type exampleEchoCommand struct{}

func (exampleEchoCommand) Summary() CommandSummary {
	return CommandSummary{Name: "echo", ShortHelp: "Echo a message"}
}

func (exampleEchoCommand) Description() CommandDescription {
	return CommandDescription{
		Name:      "echo",
		ShortHelp: "Echo a message",
		Usage:     "run echo --message <string>",
	}
}

func (exampleEchoCommand) Run(ctx context.Context, args Args) (Result, error) {
	message, _ := args.Value("message")
	return Result{OK: true, Verb: "run", Command: "echo", Output: map[string]any{"message": message}}, nil
}

func ExampleShell() {
	shell, _ := New(Config{})
	_ = shell.Register(exampleEchoCommand{})
	result, _ := shell.Execute(context.Background(), `run echo --message "hello"`)
	fmt.Println(result.OK, result.Command)
	// Output: true echo
}
