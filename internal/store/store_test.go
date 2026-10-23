package store

import (
	"path/filepath"
	"testing"
)

func TestStorePersistsTitlesAndHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yonime.sqlite")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	id, err := db.UpsertTitle(Title{
		Slug:    "12--one-piece-anime",
		EngName: "ONE PIECE",
		RusName: "Ван-Пис",
		Status:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("empty title id")
	}
	entry, err := db.EnsureHistory(id, "1", "AniLibria.TV")
	if err != nil {
		t.Fatal(err)
	}
	if entry == nil {
		t.Fatal("history entry was not created")
	}

	history, err := db.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("got %d history entries, want 1", len(history))
	}
	if history[0].RusName != "Ван-Пис" {
		t.Fatalf("history title was not decoded: %#v", history[0])
	}

	if err := db.SaveSelection(id, "2", "AniLibria.TV"); err != nil {
		t.Fatal(err)
	}
	saved, err := db.GetHistory(id)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || saved.Episode != "2" {
		t.Fatalf("saved history = %#v, want episode 2", saved)
	}

	if err := db.ReorderHistory([]int64{id}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteHistory(id); err != nil {
		t.Fatal(err)
	}
	history, err = db.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("history was not cleared: %#v", history)
	}
}

func TestUpdateNamesPreservesEmptyValues(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "yonime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	id, err := db.UpsertTitle(Title{Slug: "names-test", EngName: "Old", RusName: "Старый"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateNames(id, "", ""); err != nil {
		t.Fatal(err)
	}
	title, err := db.GetTitle("names-test")
	if err != nil {
		t.Fatal(err)
	}
	if title == nil || title.EngName != "Old" || title.RusName != "Старый" {
		t.Fatalf("empty update changed names: %#v", title)
	}

	if err := db.UpdateNames(id, "New", "Новый"); err != nil {
		t.Fatal(err)
	}
	title, err = db.GetTitle("names-test")
	if err != nil {
		t.Fatal(err)
	}
	if title == nil || title.EngName != "New" || title.RusName != "Новый" {
		t.Fatalf("names were not updated: %#v", title)
	}
}

func TestCoverCache(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "yonime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	id, err := db.UpsertTitle(Title{Slug: "cache-test", RusName: "Кэш"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveCoverFailure(id); err != nil {
		t.Fatal(err)
	}
	cover, err := db.GetCoverCache(id)
	if err != nil {
		t.Fatal(err)
	}
	if cover == nil || cover.FailedAt == 0 || cover.Filename != "" {
		t.Fatalf("failure was not recorded: %#v", cover)
	}
	if err := db.SaveCoverSuccess(id, "cover.jpg"); err != nil {
		t.Fatal(err)
	}
	cover, err = db.GetCoverCache(id)
	if err != nil {
		t.Fatal(err)
	}
	if cover == nil || cover.Filename != "cover.jpg" || cover.FailedAt != 0 {
		t.Fatalf("success was not recorded: %#v", cover)
	}
	// A failed refresh must keep the previous file so history keeps showing the
	// old cover until a replacement has been downloaded.
	if err := db.SaveCoverFailure(id); err != nil {
		t.Fatal(err)
	}
	cover, err = db.GetCoverCache(id)
	if err != nil {
		t.Fatal(err)
	}
	if cover == nil || cover.Filename != "cover.jpg" || cover.FailedAt == 0 {
		t.Fatalf("failure removed the cached filename: %#v", cover)
	}
}

func TestHistoryKeepsNewestFirst(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "yonime.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var ids []int64
	for _, slug := range []string{"first", "second", "third"} {
		id, err := db.UpsertTitle(Title{Slug: slug, RusName: slug})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		if _, err := db.EnsureHistory(id, "", ""); err != nil {
			t.Fatal(err)
		}
	}
	history, err := db.ListHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("got %d entries, want 3", len(history))
	}
	if history[0].RusName != "third" || history[2].RusName != "first" {
		t.Fatalf("unexpected order: %s, %s, %s", history[0].RusName, history[1].RusName, history[2].RusName)
	}
}
