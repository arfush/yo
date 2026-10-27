package library

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/arfush/yoanime/log"
)

// encodeVoiceQualities packs the probed Kinopoisk quality ladders into the JSON
// blob cached on the title row. AnimeLIB voiceovers carry no ladder, so an
// empty result means there is nothing worth storing.
func encodeVoiceQualities(voices []Voiceover) string {
	ladders := make(map[string][]int, len(voices))
	for _, voice := range voices {
		name := strings.TrimSpace(voice.Name)
		if name == "" || len(voice.Qualities) == 0 {
			continue
		}
		values := normalizeQualities(voice.Qualities)
		if len(values) == 0 {
			continue
		}
		ladders[name] = values
	}
	if len(ladders) == 0 {
		return ""
	}
	raw, err := json.Marshal(ladders)
	if err != nil {
		return ""
	}
	return string(raw)
}

// decodeVoiceQualities restores the per-voice ladders. A broken or stale blob
// simply yields no cached data and the title page probes the mirror again.
func decodeVoiceQualities(raw string) map[string][]int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var stored map[string][]int
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		log.Warn("cannot read cached Kinopoisk qualities", "error", err)
		return nil
	}
	ladders := make(map[string][]int, len(stored))
	for name, values := range stored {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if normalized := normalizeQualities(values); len(normalized) > 0 {
			ladders[name] = normalized
		}
	}
	if len(ladders) == 0 {
		return nil
	}
	return ladders
}

// normalizeQualities drops unusable entries and returns the ladder ascending,
// which is also the order the title page renders.
func normalizeQualities(values []int) []int {
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value > 0 {
			out = append(out, value)
		}
	}
	sort.Ints(out)
	unique := out[:0]
	for index, value := range out {
		if index > 0 && out[index-1] == value {
			continue
		}
		unique = append(unique, value)
	}
	return unique
}

// applyStoredQualities fills the voiceovers with the ladder cached when the
// title was last saved. Without a cache the title page probes the mirror once.
func (l *Library) applyStoredQualities(title *Title) {
	if l == nil || title == nil || title.ID == 0 {
		return
	}
	raw, err := l.store.GetTitleQualities(title.ID)
	if err != nil {
		log.Warn("cannot load cached Kinopoisk qualities", "title", title.ID, "error", err)
		return
	}
	ladders := decodeVoiceQualities(raw)
	if len(ladders) == 0 {
		return
	}
	for index := range title.Voiceovers {
		values := ladders[strings.TrimSpace(title.Voiceovers[index].Name)]
		if len(values) == 0 {
			continue
		}
		title.Voiceovers[index].Qualities = append([]int(nil), values...)
	}
}
