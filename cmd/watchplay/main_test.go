package main

import (
	"errors"
	"testing"

	"github.com/arfush/yonime/internal/watchparty"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    command
		wantErr bool
	}{
		{
			name: "create",
			args: []string{"create", "solo-leveling", "AniLibria", "1"},
			want: command{name: "create", args: []string{"solo-leveling", "AniLibria", "1"}},
		},
		{
			name: "open is case insensitive",
			args: []string{"OPEN", "yocbqopgz"},
			want: command{name: "open", args: []string{"yocbqopgz"}},
		},
		{name: "no arguments", args: nil, wantErr: true},
		{name: "create without episode", args: []string{"create", "slug", "voice"}, wantErr: true},
		{name: "open without key", args: []string{"open"}, wantErr: true},
		{name: "unknown", args: []string{"sync"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseCommand(test.args)
			if test.wantErr {
				if !errors.Is(err, errUsage) {
					t.Fatalf("err = %v, want errUsage", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCommand: %v", err)
			}
			if got.name != test.want.name {
				t.Fatalf("name = %q, want %q", got.name, test.want.name)
			}
			if len(got.args) != len(test.want.args) {
				t.Fatalf("args = %v, want %v", got.args, test.want.args)
			}
			for index := range got.args {
				if got.args[index] != test.want.args[index] {
					t.Fatalf("args = %v, want %v", got.args, test.want.args)
				}
			}
		})
	}
}

func TestParseSeek(t *testing.T) {
	tests := []struct {
		arg     string
		current float64
		want    float64
		wantErr bool
	}{
		{arg: "120", current: 10, want: 120},
		{arg: "0", current: 300, want: 0},
		{arg: "+30", current: 100, want: 130},
		{arg: "-15", current: 100, want: 85},
		{arg: "-500", current: 100, want: 0},
		{arg: "12,5", current: 0, want: 12.5},
		{arg: "abc", current: 0, wantErr: true},
		{arg: "", current: 0, wantErr: true},
	}
	for _, test := range tests {
		got, err := parseSeek(test.arg, test.current)
		if test.wantErr {
			if err == nil {
				t.Fatalf("parseSeek(%q) = %v, want an error", test.arg, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseSeek(%q): %v", test.arg, err)
		}
		if got != test.want {
			t.Fatalf("parseSeek(%q) = %v, want %v", test.arg, got, test.want)
		}
	}
}

// TestCleanConfigDropsRepositoryPlaceholders keeps a source build from dialling
// the example relay that ships in watchplay.json.
func TestCleanConfigDropsRepositoryPlaceholders(t *testing.T) {
	example := watchparty.Config{
		CloudflareURL: "wss://yonime-watchparty.example.workers.dev",
		SupabaseURL:   "wss://abcdefghijkl.supabase.co",
		SupabaseKey:   "sb_publishable_...",
	}
	if usable(cleanConfig(example)) {
		t.Fatal("placeholders must not count as a configured relay")
	}

	real := watchparty.Config{
		CloudflareURL: "wss://yonime-watchparty.me.workers.dev",
	}
	cleaned := cleanConfig(real)
	if !usable(cleaned) {
		t.Fatal("a real relay address must survive cleaning")
	}
	if cleaned.CloudflareURL != real.CloudflareURL {
		t.Fatalf("url = %q, want %q", cleaned.CloudflareURL, real.CloudflareURL)
	}
}

func TestChooseProvider(t *testing.T) {
	onlySupabase := watchparty.Config{
		SupabaseURL: "wss://project.supabase.co",
		SupabaseKey: "sb_publishable_example",
	}
	provider, err := chooseProvider(onlySupabase, "")
	if err != nil {
		t.Fatalf("chooseProvider: %v", err)
	}
	if provider != watchparty.Supabase {
		t.Fatalf("provider = %q, want supabase", provider)
	}
	if _, err := chooseProvider(onlySupabase, "cloudflare"); err == nil {
		t.Fatal("an unconfigured relay must be reported")
	}
}
