package party

import (
	"strings"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	key := deriveContentKey("YOC4K7Q2M", "4K7Q2M9XBT", make([]byte, saltSize))
	sealed, err := sealContent(key, []byte(`{"state":{"title":"demo"}}`))
	if err != nil {
		t.Fatalf("sealContent: %v", err)
	}
	if strings.Contains(sealed, "demo") {
		t.Fatalf("sealed payload leaks the plaintext: %s", sealed)
	}
	opened, err := openContent(key, sealed)
	if err != nil {
		t.Fatalf("openContent: %v", err)
	}
	if string(opened) != `{"state":{"title":"demo"}}` {
		t.Fatalf("openContent = %q", opened)
	}
}

func TestOpenRejectsAnotherKeyAndGarbage(t *testing.T) {
	key := deriveContentKey("YOC4K7Q2M", "", make([]byte, saltSize))
	sealed, err := sealContent(key, []byte("payload"))
	if err != nil {
		t.Fatalf("sealContent: %v", err)
	}
	other := deriveContentKey("YOC4K7Q2M", "4K7Q2M9XBT", make([]byte, saltSize))
	if _, err := openContent(other, sealed); err == nil {
		t.Fatal("another key opened the payload")
	}
	if _, err := openContent(key, "not base64!"); err == nil {
		t.Fatal("garbage decoded")
	}
	if _, err := openContent(key, ""); err == nil {
		t.Fatal("an empty payload decoded")
	}
	short, err := sealContent(key, nil)
	if err != nil {
		t.Fatalf("sealContent: %v", err)
	}
	if _, err := openContent(key, short); err != nil {
		t.Fatalf("an empty plaintext must round-trip: %v", err)
	}
}

func TestCardProvesTheSecret(t *testing.T) {
	secret := "4K7Q2M9XBT"
	card, key, err := newCard("YOC4K7Q2M", secret, true, 1)
	if err != nil {
		t.Fatalf("newCard: %v", err)
	}
	if !card.Valid() {
		t.Fatal("a fresh card is not valid")
	}
	if !card.Locked || card.Epoch != 1 || card.Salt == "" || card.Check == "" {
		t.Fatalf("unexpected card: %+v", card)
	}
	got, ok := card.Key("YOC4K7Q2M", secret)
	if !ok {
		t.Fatal("the right secret did not open the card")
	}
	if string(got) != string(key) {
		t.Fatal("card.Key returned another key")
	}
	if _, ok := card.Key("YOC4K7Q2M", "0000000000"); ok {
		t.Fatal("a wrong secret opened the card")
	}
	if _, ok := card.Key("YOC4K7Q2N", secret); ok {
		t.Fatal("another room key opened the card")
	}
	if _, ok := (Card{Locked: true, Epoch: 1, Salt: "%%", Check: "x"}).Key("YOC4K7Q2M", secret); ok {
		t.Fatal("a corrupt salt was accepted")
	}
	if (Card{}).Valid() {
		t.Fatal("an empty card is valid")
	}
}

func TestCardRotationChangesTheSalt(t *testing.T) {
	first, _, err := newCard("YOC4K7Q2M", "4K7Q2M9XBT", true, 1)
	if err != nil {
		t.Fatalf("newCard: %v", err)
	}
	// The same secret must not derive the same key after a rotation: a new
	// salt guarantees a fresh key even if the value repeats.
	second, _, err := newCard("YOC4K7Q2M", "4K7Q2M9XBT", true, 2)
	if err != nil {
		t.Fatalf("newCard: %v", err)
	}
	if first.Salt == second.Salt {
		t.Fatal("two cards reused the salt")
	}
	if first.Check == second.Check {
		t.Fatal("two cards produced the same check")
	}
	if _, ok := first.Key("YOC4K7Q2M", "4K7Q2M9XBT"); !ok {
		t.Fatal("the first card stopped working")
	}
	if _, ok := second.Key("YOC4K7Q2M", "4K7Q2M9XBT"); !ok {
		t.Fatal("the second card did not work")
	}
}

// TestKDFDefaultsAreDeliberatelySlow pins the parameters the tests replace: a
// release has to pay real memory and time per guess, or a leaked card becomes
// brute-forceable.
func TestKDFDefaultsAreDeliberatelySlow(t *testing.T) {
	if kdfTimeDefault < 3 {
		t.Fatalf("kdfTimeDefault = %d, want at least 3", kdfTimeDefault)
	}
	if kdfMemoryDefault < 64*1024 {
		t.Fatalf("kdfMemoryDefault = %d KiB, want at least 64 MiB", kdfMemoryDefault)
	}
	if kdfThreadsDefault < 1 {
		t.Fatalf("kdfThreadsDefault = %d", kdfThreadsDefault)
	}
}
