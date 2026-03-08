package parser

import (
	"reflect"
	"testing"
)

func TestParseDiscover(t *testing.T) {
	parsed, err := ParseLine(`discover weather alerts`)
	if err != nil {
		t.Fatalf("ParseLine returned error: %v", err)
	}
	if parsed.Verb != VerbDiscover {
		t.Fatalf("unexpected verb: %q", parsed.Verb)
	}
	if parsed.Query != "weather alerts" {
		t.Fatalf("unexpected query: %q", parsed.Query)
	}
}

func TestParseDescribe(t *testing.T) {
	parsed, err := ParseLine(`describe weather`)
	if err != nil {
		t.Fatalf("ParseLine returned error: %v", err)
	}
	if parsed.Command != "weather" {
		t.Fatalf("unexpected command: %q", parsed.Command)
	}
}

func TestParseRunFlags(t *testing.T) {
	parsed, err := ParseLine(`run write --path=/tmp/file --message "hello world" --tag one --tag two`)
	if err != nil {
		t.Fatalf("ParseLine returned error: %v", err)
	}
	want := map[string][]string{
		"path":    {"/tmp/file"},
		"message": {"hello world"},
		"tag":     {"one", "two"},
	}
	if !reflect.DeepEqual(parsed.Flags, want) {
		t.Fatalf("unexpected flags: %#v", parsed.Flags)
	}
}

func TestParseRejectsMissingFlagValue(t *testing.T) {
	if _, err := ParseLine(`run weather --city`); err == nil {
		t.Fatal("expected error for missing flag value")
	}
}

func TestParseRejectsMultiline(t *testing.T) {
	if _, err := ParseLine("run weather\n--city sf"); err == nil {
		t.Fatal("expected error for multiline input")
	}
}
