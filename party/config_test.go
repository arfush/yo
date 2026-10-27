package party

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "party.json")
	cfg := Config{
		CloudflareURL: " https://party.example.workers.dev/ ",
		SupabaseURL:   "https://abcd.supabase.co/",
		SupabaseKey:   " sb_publishable_example ",
	}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	if !strings.Contains(string(raw), `"cloudflare_endpoint"`) {
		t.Fatalf("saved file has no cloudflare_endpoint: %s", raw)
	}

	loaded := LoadConfig(path)
	if loaded.CloudflareURL != "wss://party.example.workers.dev" {
		t.Fatalf("cloudflare url = %q", loaded.CloudflareURL)
	}
	if loaded.SupabaseURL != "wss://abcd.supabase.co" {
		t.Fatalf("supabase url = %q", loaded.SupabaseURL)
	}
	if loaded.SupabaseKey != "sb_publishable_example" {
		t.Fatalf("supabase key = %q", loaded.SupabaseKey)
	}
}

func TestConfigFileOverridesBuiltInRelay(t *testing.T) {
	oldCloudflare := CloudflareURL
	CloudflareURL = "wss://built-in.example.workers.dev"
	t.Cleanup(func() { CloudflareURL = oldCloudflare })

	path := filepath.Join(t.TempDir(), "party.json")
	if err := os.WriteFile(path, []byte(`{"cloudflare_endpoint":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := LoadConfig(path).CloudflareURL; got != "" {
		t.Fatalf("cloudflare url = %q, want the empty file value to win", got)
	}
}

func TestLoadConfigAcceptsLegacyRelayKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "party.json")
	raw := `{"cloudflare_url":"https://old.example.workers.dev","supabase_url":"https://abcd.supabase.co","supabase_key":"sb_publishable_old"}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	got := LoadConfig(path)
	if got.CloudflareURL != "wss://old.example.workers.dev" ||
		got.SupabaseURL != "wss://abcd.supabase.co" ||
		got.SupabaseKey != "sb_publishable_old" {
		t.Fatalf("legacy config = %+v", got)
	}
}

func TestLoadConfigWithoutFileUsesBuiltInRelay(t *testing.T) {
	oldCloudflare := CloudflareURL
	CloudflareURL = "wss://built-in.example.workers.dev"
	t.Cleanup(func() { CloudflareURL = oldCloudflare })

	got := LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if got.CloudflareURL != CloudflareURL {
		t.Fatalf("cloudflare url = %q, want the built-in value", got.CloudflareURL)
	}
}

func TestConfigValidation(t *testing.T) {
	valid := []Config{
		{},
		{CloudflareURL: "https://party.example.workers.dev"},
		{SupabaseURL: "wss://abcd.supabase.co"},
		{SupabaseURL: "wss://abcd.supabase.co", SupabaseKey: "sb_publishable_example"},
		{SupabaseKey: "sb_publishable_example"},
	}
	for _, cfg := range valid {
		if err := cfg.Validate(); err != nil {
			t.Fatalf("Validate(%+v) = %v, want nil", cfg, err)
		}
	}

	invalid := []Config{
		{CloudflareURL: "party.example.workers.dev"},
		{SupabaseURL: "https://"},
		{CloudflareURL: "ftp://party.example.workers.dev"},
		{SupabaseURL: "wss://abcd.supabase.co", SupabaseKey: "sb_secret_example"},
		{SupabaseURL: "wss://abcd.supabase.co", SupabaseKey: "service_role"},
	}
	for _, cfg := range invalid {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("Validate(%+v) = nil, want an error", cfg)
		}
	}
}
