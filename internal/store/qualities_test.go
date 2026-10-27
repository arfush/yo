package store

import (
	"path/filepath"
	"testing"
)

func TestTitleQualitiesCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yoanime.sqlite")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	id, err := db.UpsertTitle(Title{Slug: "kinopoisk:88198", Source: SourceKinopoisk, ExternalID: "88198"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := db.GetTitleQualities(id); err != nil || got != "" {
		t.Fatalf("empty cache read = %q, %v", got, err)
	}
	const data = `{"Гаврилов":[360,480,720,1080]}`
	if err := db.SaveTitleQualities(id, data); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetTitleQualities(id)
	if err != nil {
		t.Fatal(err)
	}
	if got != data {
		t.Fatalf("cached ladder = %q, want %q", got, data)
	}
	// A broken write must never erase the cached ladder.
	if err := db.SaveTitleQualities(id, ""); err != nil {
		t.Fatal(err)
	}
	if got, err := db.GetTitleQualities(id); err != nil || got != data {
		t.Fatalf("empty write clobbered the cache: %q, %v", got, err)
	}
	// Deleting the history row keeps the cached metadata (and with it the
	// ladder), so a re-added title does not have to probe again.
	if err := db.DeleteHistory(id); err != nil {
		t.Fatal(err)
	}
	if got, err := db.GetTitleQualities(id); err != nil || got != data {
		t.Fatalf("cache was lost with history: %q, %v", got, err)
	}
}

func TestTitleQualitiesIgnoreUnknownTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yoanime.sqlite")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.SaveTitleQualities(0, `{"x":[1080]}`); err != nil {
		t.Fatalf("save with zero id: %v", err)
	}
	if got, err := db.GetTitleQualities(999); err != nil || got != "" {
		t.Fatalf("read with unknown id = %q, %v", got, err)
	}
}

func TestGenericSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yoanime.sqlite")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if got, ok := db.Settings("window_rect"); ok || got != "" {
		t.Fatalf("missing setting read = %q, %v", got, ok)
	}
	if err := db.SetSettings("window_rect", "120,40,960,540"); err != nil {
		t.Fatal(err)
	}
	got, ok := db.Settings("window_rect")
	if !ok || got != "120,40,960,540" {
		t.Fatalf("setting read = %q, %v", got, ok)
	}
	if err := db.SetSettings("window_rect", "200,100,800,600"); err != nil {
		t.Fatal(err)
	}
	if got, _ := db.Settings("window_rect"); got != "200,100,800,600" {
		t.Fatalf("updated setting = %q", got)
	}
	if err := db.SetSettings("", "x"); err != nil {
		t.Fatalf("empty key must be a no-op: %v", err)
	}
}
