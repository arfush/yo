package watchparty

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NameLength is the longest custom nickname, in runes.
const NameLength = 24

// ErrInvalidName reports a nickname that cannot be shown in a room.
var ErrInvalidName = errors.New("некорректный никнейм")

// The default name is a GitHub-style adjective-noun pair. Two English words are
// short enough for the room card, easy to read in a chat and do not look like a
// machine identifier the way "YOU-4F2A" did.
var nameAdjectives = []string{
	"quiet", "brave", "sunny", "salty", "nimble", "sleepy", "frosty", "velvet",
	"cosmic", "misty", "lucky", "wild", "calm", "sharp", "gentle", "rusty",
	"golden", "silver", "shadow", "amber", "brisk", "clever", "dusty", "eager",
	"fancy", "fuzzy", "glossy", "happy", "hidden", "humble", "icy", "jolly",
	"keen", "lazy", "lively", "lunar", "merry", "mighty", "mossy", "mellow",
	"neon", "noble", "plucky", "polar", "proud", "quick", "rapid", "rosy",
	"royal", "shy", "silent", "smooth", "snowy", "solar", "spicy", "steady",
	"stormy", "subtle", "swift", "tidy", "vivid", "warm", "witty", "young",
}

var nameNouns = []string{
	"tanuki", "otter", "fox", "panda", "badger", "beaver", "bison", "bobcat",
	"camel", "cheetah", "condor", "coyote", "crane", "crow", "deer", "dingo",
	"dolphin", "donkey", "eagle", "falcon", "ferret", "finch", "gecko", "goose",
	"hamster", "hare", "hawk", "heron", "husky", "ibis", "jackal", "jaguar",
	"koala", "lemur", "leopard", "lizard", "llama", "lynx", "magpie", "marten",
	"mole", "moose", "moth", "narwhal", "newt", "ocelot", "orca", "osprey",
	"owl", "parrot", "penguin", "puffin", "quail", "rabbit", "raven", "seal",
	"shrew", "sloth", "sparrow", "stoat", "swan", "tiger", "walrus", "wombat",
}

// NewName returns a random default nickname, such as quiet-tanuki.
func NewName() (string, error) {
	adjective, err := randomIndex(len(nameAdjectives))
	if err != nil {
		return "", err
	}
	noun, err := randomIndex(len(nameNouns))
	if err != nil {
		return "", err
	}
	return nameAdjectives[adjective] + "-" + nameNouns[noun], nil
}

// randomIndex draws a uniform index below n without modulo bias.
func randomIndex(n int) (int, error) {
	if n <= 0 || n > 256 {
		return 0, fmt.Errorf("bad random range %d", n)
	}
	limit := 256 - 256%n
	buf := make([]byte, 1)
	for {
		if _, err := rand.Read(buf); err != nil {
			return 0, err
		}
		if int(buf[0]) < limit {
			return int(buf[0]) % n, nil
		}
	}
}

// NormalizeName trims a custom nickname and rejects values that cannot be shown
// next to a room. A string that parses as a room key is refused too: the two
// are shared in the same chat message, and a nickname that looks like a key
// would send a guest to the wrong field.
func NormalizeName(value string) (string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", ErrInvalidName
	}
	if utf8.RuneCountInString(name) > NameLength {
		return "", ErrInvalidName
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", ErrInvalidName
		}
	}
	if _, err := NormalizeKey(name); err == nil {
		return "", ErrInvalidName
	}
	return name, nil
}
