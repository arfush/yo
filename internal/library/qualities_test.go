package library

import (
	"reflect"
	"testing"
)

func TestVoiceQualityLadderRoundTrip(t *testing.T) {
	voices := []Voiceover{
		{Name: "Гаврилов", Episodes: []string{"1"}, Qualities: []int{1080, 720, 360, 480}},
		{Name: "Субтитры", Episodes: []string{"1"}, Qualities: []int{1080, 1080, 2160}},
		{Name: "Пусто", Episodes: []string{"1"}},
	}
	raw := encodeVoiceQualities(voices)
	if raw == "" {
		t.Fatal("encodeVoiceQualities returned empty for a voice with a ladder")
	}
	ladders := decodeVoiceQualities(raw)
	want := map[string][]int{
		"Гаврилов": {360, 480, 720, 1080},
		"Субтитры": {1080, 2160},
	}
	if !reflect.DeepEqual(ladders, want) {
		t.Fatalf("ladders = %v, want %v", ladders, want)
	}

	if got := encodeVoiceQualities([]Voiceover{{Name: "Аниме"}}); got != "" {
		t.Fatalf("encode with no ladder = %q, want empty", got)
	}
	if got := decodeVoiceQualities(""); got != nil {
		t.Fatalf("decode of empty = %v, want nil", got)
	}
	if got := decodeVoiceQualities("not json"); got != nil {
		t.Fatalf("decode of garbage = %v, want nil", got)
	}
	if got := decodeVoiceQualities(`{"x":[0,-5,1080,360,360]}`); !reflect.DeepEqual(got, map[string][]int{"x": {360, 1080}}) {
		t.Fatalf("decode cleaning = %v", got)
	}
}

func TestNormalizeQualities(t *testing.T) {
	if got := normalizeQualities([]int{2160, 720, 0, 1080, 720, -1}); !reflect.DeepEqual(got, []int{720, 1080, 2160}) {
		t.Fatalf("normalizeQualities = %v", got)
	}
	if got := normalizeQualities(nil); len(got) != 0 {
		t.Fatalf("normalize of nil = %v", got)
	}
}
