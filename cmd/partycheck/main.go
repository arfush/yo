// Command partycheck exercises the real watch-party relay without starting
// Gio or mpv. It creates a host room, joins it as a guest, publishes the three
// content shapes used by yo (anime episode, series episode and a Filmlib movie)
// and verifies the anchor and shared-control round trip.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/arfush/yo/party"
)

type preset struct {
	Name     string
	Slug     string
	Title    string
	Voice    string
	Episode  string
	Index    int
	Position float64
}

type result struct {
	Case      string `json:"case"`
	Slug      string `json:"slug"`
	Room      string `json:"room"`
	JoinMS    int64  `json:"join_ms"`
	AnchorMS  int64  `json:"anchor_ms"`
	DriftMS   int64  `json:"drift_ms"`
	ControlMS int64  `json:"control_ms"`
	Passed    bool   `json:"passed"`
	Error     string `json:"error,omitempty"`
}

func main() {
	configPath := flag.String("config", defaultConfigPath(), "party.json path")
	providerName := flag.String("provider", string(party.Cloudflare), "relay: cloudflare or supabase")
	timeout := flag.Duration("timeout", 30*time.Second, "timeout for one case")
	jsonOutput := flag.Bool("json", false, "print one JSON object per line")
	flag.Parse()

	provider := party.Provider(strings.ToLower(strings.TrimSpace(*providerName)))
	if !provider.Valid() {
		fail("unknown provider %q (want cloudflare or supabase)", *providerName)
	}
	cfg := party.LoadConfig(*configPath)
	if !cfg.Configured(provider) {
		fail("%s relay is not configured in %s", provider.Label(), *configPath)
	}

	presets := []preset{
		{
			Name:     "anime",
			Slug:     "cli-anime-episode",
			Title:    "CLI anime",
			Voice:    "Anime voice",
			Episode:  "1",
			Index:    0,
			Position: 12.5,
		},
		{
			Name:     "series",
			Slug:     "cli-series-episode",
			Title:    "CLI series",
			Voice:    "Series voice",
			Episode:  "S1E2",
			Index:    1,
			Position: 301.25,
		},
		{
			Name:     "filmlib-movie",
			Slug:     "filmlib:12345",
			Title:    "CLI Filmlib movie",
			Voice:    "Alloha",
			Episode:  "1",
			Index:    0,
			Position: 61.75,
		},
	}

	results := make([]result, 0, len(presets))
	failed := 0
	for _, item := range presets {
		got := runCase(cfg, provider, *timeout, item)
		results = append(results, got)
		if !got.Passed {
			failed++
		}
		if !*jsonOutput {
			printResult(got)
		}
	}

	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		for _, item := range results {
			_ = encoder.Encode(item)
		}
	}
	fmt.Printf("partycheck provider=%s passed=%d/%d\n", provider, len(results)-failed, len(results))
	if failed > 0 {
		os.Exit(1)
	}
}

func runCase(cfg party.Config, provider party.Provider, timeout time.Duration, item preset) result {
	started := time.Now()
	out := result{Case: item.Name, Slug: item.Slug}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	host, err := party.Create(ctx, cfg, provider, "partycheck-host")
	if err != nil {
		out.Error = fmt.Sprintf("create room: %v", err)
		return out
	}
	defer host.Close()
	out.Room = host.Key

	guest, err := party.Join(ctx, cfg, host.Key, "partycheck-guest")
	if err != nil {
		out.Error = fmt.Sprintf("join room: %v", err)
		return out
	}
	defer guest.Close()

	if err := guest.WaitHost(ctx); err != nil {
		out.Error = fmt.Sprintf("wait host: %v", err)
		return out
	}
	if err := guest.WaitCard(ctx); err != nil {
		out.Error = fmt.Sprintf("wait card: %v", err)
		return out
	}
	out.JoinMS = time.Since(started).Milliseconds()

	control := true
	want := party.State{
		TitleSlug:      item.Slug,
		TitleName:      item.Title,
		Voice:          item.Voice,
		Episode:        item.Episode,
		EpisodeIndex:   item.Index,
		Playing:        true,
		Position:       item.Position,
		ControlEnabled: &control,
	}
	anchorAt := time.Now()
	if err := host.Publish(want); err != nil {
		out.Error = fmt.Sprintf("publish anchor: %v", err)
		return out
	}
	got, err := waitGuestState(ctx, guest, item, true)
	if err != nil {
		out.Error = fmt.Sprintf("receive anchor: %v", err)
		return out
	}
	out.AnchorMS = time.Since(anchorAt).Milliseconds()
	out.DriftMS = int64(math.Round((got.PositionAt(time.Now()) - want.PositionAt(time.Now())) * 1000))

	controlAt := time.Now()
	if err := guest.SendControl(party.ControlPause, 0); err != nil {
		out.Error = fmt.Sprintf("send pause control: %v", err)
		return out
	}
	if err := waitHostControl(ctx, host, party.ControlPause); err != nil {
		out.Error = fmt.Sprintf("receive pause control: %v", err)
		return out
	}
	paused := want
	paused.Playing = false
	paused.Position = want.PositionAt(time.Now())
	if err := host.Publish(paused); err != nil {
		out.Error = fmt.Sprintf("publish paused anchor: %v", err)
		return out
	}
	if _, err := waitGuestState(ctx, guest, item, false); err != nil {
		out.Error = fmt.Sprintf("receive paused anchor: %v", err)
		return out
	}
	out.ControlMS = time.Since(controlAt).Milliseconds()
	out.Passed = true
	return out
}

func waitGuestState(ctx context.Context, guest *party.Session, item preset, playing bool) (party.State, error) {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		if state, ok := guest.State(); ok && state.SamePlayback(party.State{
			TitleSlug: item.Slug,
			Voice:     item.Voice,
			Episode:   item.Episode,
		}) && state.Playing == playing {
			return state, nil
		}
		select {
		case <-ctx.Done():
			return party.State{}, ctx.Err()
		case <-deadline.C:
			return party.State{}, fmt.Errorf("timed out waiting for %s anchor", item.Name)
		case <-guest.Updates():
			// Re-read State after every notification. Updates is a hint; the
			// newest anchor is always available on the session.
		}
	}
}

func waitHostControl(ctx context.Context, host *party.Session, action string) error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for control %q", action)
		case update := <-host.Updates():
			if update.Kind == party.UpdateControl && update.Control.Action == action {
				return nil
			}
			if update.Kind == party.UpdateError && update.Err != nil {
				return update.Err
			}
		}
	}
}

func printResult(item result) {
	status := "ok"
	if !item.Passed {
		status = "FAIL"
	}
	fmt.Printf("%-14s %-4s room=%s join=%dms anchor=%dms drift=%dms control=%dms",
		item.Case, status, item.Room, item.JoinMS, item.AnchorMS, item.DriftMS, item.ControlMS)
	if item.Error != "" {
		fmt.Printf(" error=%s", item.Error)
	}
	fmt.Println()
}

func defaultConfigPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if dir, err := os.UserConfigDir(); err == nil {
			base = dir
		}
	}
	if base == "" {
		return "party.json"
	}
	return filepath.Join(base, "yoanime", "party.json")
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "partycheck: "+format+"\n", args...)
	os.Exit(2)
}
