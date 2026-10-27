package library

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/arfush/yoanime/internal/kinopoisk"
	"github.com/arfush/yoanime/internal/store"
	"github.com/arfush/yoanime/log"
)

// loadKinopoisk builds a title from the mirror catalogue. persist mirrors the
// AnimeLIB split between Load (a title already in history) and Fetch (a link
// typed into the URL field): Fetch returns local ids and the saved selection but
// does not write the title or create a history row.
func (l *Library) loadKinopoisk(ctx context.Context, link kinopoisk.Link, persist bool) (Title, error) {
	action := "fetching"
	if persist {
		action = "loading"
	}
	log.Info(action+" Kinopoisk title", "title", link.ID)

	client := l.kinopoisk
	if client == nil {
		client = kinopoisk.NewClient()
	}
	meta, err := client.Meta(ctx, link)
	if err != nil {
		log.Error("cannot load Kinopoisk metadata", "title", link.ID, "error", err)
		return Title{}, err
	}
	voices, err := client.Voices(ctx, link.ID)
	if err != nil {
		if ctx.Err() != nil {
			return Title{}, ctx.Err()
		}
		log.Error("cannot load Kinopoisk voices", "title", link.ID, "error", err)
		return Title{}, err
	}
	catalog, err := loadKinopoiskCatalog(ctx, client, voices)
	if err != nil {
		if ctx.Err() != nil {
			return Title{}, ctx.Err()
		}
		log.Error("cannot load Kinopoisk catalogue", "title", link.ID, "error", err)
		return Title{}, err
	}

	title := kinopoiskTitle(meta, catalog, voices)
	existing, err := l.store.GetTitle(title.Slug)
	if err != nil {
		return Title{}, err
	}
	if existing != nil {
		applyStoredKinopoisk(&title, *existing)
		l.applyStoredQualities(&title)
	} else {
		title.UserStatus = UserStatusPlanned
	}

	if persist {
		stored, err := l.upsertLoadedTitle(storedTitleFromLibrary(title))
		if err != nil {
			return Title{}, err
		}
		applyStoredKinopoisk(&title, stored)
		l.applyStoredQualities(&title)
		if err := l.restoreSelection(&title); err != nil {
			return Title{}, err
		}
	} else {
		title.ApplySelection()
		if existing != nil {
			if entry, historyErr := l.store.GetHistory(existing.ID); historyErr == nil && entry != nil {
				title.SelectedVoice, title.SelectedEpisode = chooseSelection(title, entry.Voiceover, entry.Episode)
				title.ResumeAt = entry.ResumeAt
				title.Duration = entry.Duration
			}
		}
	}

	l.ensureCover(ctx, &title)
	log.Info("Kinopoisk title ready", "title", title.Russian, "slug", title.Slug, "episodes", len(title.Episodes), "seasons", len(title.Seasons), "voices", len(title.Voiceovers))
	return title, nil
}

// loadKinopoiskCatalog tries the first few translations until one returns the
// file list. Every translation page embeds the whole catalogue, so one hit is
// enough; trying more guards against a single broken player page.
func loadKinopoiskCatalog(ctx context.Context, client *kinopoisk.Client, voices []kinopoisk.Voice) (kinopoisk.Catalog, error) {
	var lastErr error
	for index, voice := range voices {
		if index >= 3 {
			break
		}
		catalog, err := client.Catalog(ctx, voice)
		if err == nil && len(catalog.Episodes) > 0 {
			return catalog, nil
		}
		if err != nil {
			lastErr = err
		}
		if ctx.Err() != nil {
			return kinopoisk.Catalog{}, ctx.Err()
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("плеер вернул пустой список серий")
	}
	return kinopoisk.Catalog{}, fmt.Errorf("не удалось прочитать серии: %w", lastErr)
}

// kinopoiskTitle converts the catalogue into the same model the AnimeLIB path
// produces: a flat episode list plus one voiceover per translation.
func kinopoiskTitle(meta kinopoisk.Meta, catalog kinopoisk.Catalog, voices []kinopoisk.Voice) Title {
	title := Title{
		Source:        kinopoisk.Source,
		ExternalID:    strconv.Itoa(meta.ID),
		Slug:          kinopoisk.SlugPrefix + strconv.Itoa(meta.ID),
		Russian:       strings.TrimSpace(meta.NameRu),
		Original:      strings.TrimSpace(meta.NameEn),
		Status:        StatusCompleted,
		IsMovie:       catalog.Movie,
		CoverURL:      meta.PosterURL,
		Episodes:      append([]string(nil), catalog.Episodes...),
		EpisodesTotal: len(catalog.Episodes),
		IMDbID:        strings.TrimSpace(meta.IMDbID),
		Voiceovers:    []Voiceover{},
	}
	if !meta.Completed && !catalog.Movie {
		title.Status = StatusOngoing
	}
	if title.Russian == "" {
		title.Russian = title.Original
	}
	if title.Original == title.Russian {
		title.Original = ""
	}

	// A film is a single file, not a season: the catalogue reports it as
	// season zero, which would not match the flat "1" episode key and would
	// put a phantom season chip into the UI.
	if !catalog.Movie {
		for _, season := range catalog.Seasons {
			entry := Season{Number: season.Number}
			for _, number := range season.Episodes {
				entry.Episodes = append(entry.Episodes, kinopoisk.EpisodeKey(season.Number, number))
			}
			if len(entry.Episodes) > 0 {
				title.Seasons = append(title.Seasons, entry)
			}
		}
	}

	// Voices with the same display name are indistinguishable in the UI, so the
	// one that covers more episodes wins.
	position := make(map[string]int)
	for _, voice := range voices {
		name := strings.TrimSpace(voice.Name)
		if name == "" {
			name = fmt.Sprintf("Озвучка %d", voice.ID)
		}
		episodes := catalog.Available[voice.ID]
		if len(episodes) == 0 {
			continue
		}
		voiceover := Voiceover{
			Name:     name,
			Episodes: append([]string(nil), episodes...),
			Missing:  missingEpisodes(catalog.Episodes, episodes),
		}
		if index, ok := position[name]; ok {
			if len(title.Voiceovers[index].Episodes) >= len(voiceover.Episodes) {
				continue
			}
			title.Voiceovers[index] = voiceover
			continue
		}
		position[name] = len(title.Voiceovers)
		title.Voiceovers = append(title.Voiceovers, voiceover)
	}
	return title
}

// missingEpisodes returns the catalogue keys a voice does not carry.
func missingEpisodes(all, available []string) []string {
	if len(available) >= len(all) {
		return nil
	}
	present := make(map[string]bool, len(available))
	for _, key := range available {
		present[key] = true
	}
	missing := make([]string, 0, len(all)-len(available))
	for _, key := range all {
		if !present[key] {
			missing = append(missing, key)
		}
	}
	return missing
}

// applyStoredKinopoisk restores the local identity and user choice that the
// network response cannot know. A fallback name produced after a metadata
// outage must not replace a real stored name.
func applyStoredKinopoisk(title *Title, stored store.Title) {
	if title == nil {
		return
	}
	title.ID = stored.ID
	if stored.Source != "" {
		title.Source = stored.Source
	}
	if stored.ExternalID != "" {
		title.ExternalID = stored.ExternalID
	}
	if stored.RusName != "" && !hasRealKinopoiskName(*title) {
		title.Russian = stored.RusName
	}
	if stored.EngName != "" && stored.EngName != title.Russian {
		title.Original = stored.EngName
	}
	if strings.TrimSpace(title.Original) == strings.TrimSpace(title.Russian) {
		title.Original = ""
	}
	if stored.IMDbID != "" {
		title.IMDbID = stored.IMDbID
	}
	title.UserStatus = NormalizeUserStatus(UserStatus(stored.UserStatus))
	if stored.EpisodesTotal > 0 {
		title.EpisodesTotal = stored.EpisodesTotal
	}
	if title.Status == StatusAnnounced {
		title.UserStatus = UserStatusPlanned
	}
}

// hasRealKinopoiskName reports whether the metadata request returned a name
// instead of the "Кинопоиск #id" placeholder.
func hasRealKinopoiskName(title Title) bool {
	name := strings.TrimSpace(title.Russian)
	return name != "" && !strings.HasPrefix(name, "Кинопоиск #")
}
