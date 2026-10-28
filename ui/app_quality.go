package ui

import (
	"context"

	"gioui.org/layout"
	"gioui.org/op"

	"github.com/arfush/yo/filmlib"
)

// qualityResult is one background quality probe. slug identifies the title page
// that asked for it, so a late result cannot repaint another title.
type qualityResult struct {
	slug   string
	voice  string
	values []int
	err    error
}

func (a *application) processQualityResults(gtx layout.Context) {
	for {
		select {
		case result := <-a.qualityResults:
			a.applyQualityResult(result)
			gtx.Execute(op.InvalidateCmd{})
		default:
			return
		}
	}
}

// applyQualityResult hands a probed ladder to the title page that asked for it.
// A stale result (the user already navigated away) is dropped.
func (a *application) applyQualityResult(result qualityResult) {
	title, ok := a.active.(*titleScreen)
	if !ok || title.item.slug != result.slug {
		return
	}
	if result.err != nil {
		// The probe is a nicety: the standard quality buttons stay enabled and
		// playback still picks the nearest available step.
		logger.Warn("filmlib quality probe failed", "title", result.slug, "voice", result.voice, "error", result.err)
		return
	}
	index := -1
	for voiceIndex, voice := range title.item.voices {
		if voice.name == result.voice {
			index = voiceIndex
			break
		}
	}
	if index < 0 {
		return
	}
	title.applyQualities(index, result.values)
}

// probeQualities learns which stream qualities the selected Filmlib voice
// offers and disables the buttons it cannot deliver. The probe runs in the
// background with the shared headless Chromium and never opens a window; its
// result is cached when the user saves the title to history.
func (a *application) probeQualities(title *titleScreen) {
	if a.library == nil || a.playback == nil || title == nil {
		return
	}
	if title.item.source != filmlib.Source {
		return
	}
	index := title.selectedVoice
	if index < 0 || index >= len(title.item.voices) {
		return
	}
	voice := title.item.voices[index]
	if len(voice.qualities) > 0 || len(voice.episodes) == 0 {
		return
	}
	item := title.item
	libraryTitle := libraryTitleFromHistory(item)
	voiceName := voice.name
	episode := voice.episodes[0]
	preferred := a.library.PreferredQuality()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), qualitiesProbeTimeout)
		defer cancel()
		plan, err := a.library.PlaybackPlan(ctx, libraryTitle, voiceName, episode)
		var values []int
		if err == nil {
			values, err = a.playback.Qualities(ctx, plan, preferred)
		}
		select {
		case a.qualityResults <- qualityResult{slug: item.slug, voice: voiceName, values: values, err: err}:
		default:
		}
		if a.window != nil {
			a.window.Invalidate()
		}
	}()
}
