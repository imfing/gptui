package chat

import "unicode"

// countTokens counts the approximate number of tokens from the given text
func countTokens(text string) int {
	tokenCount := 0
	isPrevSpace := true

	for _, r := range text {
		if unicode.IsSpace(r) {
			isPrevSpace = true
		} else {
			if isPrevSpace {
				tokenCount++
			}
			isPrevSpace = false
		}
	}

	return tokenCount
}
