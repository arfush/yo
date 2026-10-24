package ui

import "unicode"

// upperFirst capitalises the first letter of a user-facing message. Internal
// errors are written in lower case by convention, but the UI shows them as
// standalone sentences.
func upperFirst(text string) string {
	runes := []rune(text)
	for index, r := range runes {
		if unicode.IsLetter(r) {
			runes[index] = unicode.ToUpper(r)
			break
		}
	}
	return string(runes)
}
