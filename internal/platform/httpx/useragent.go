package httpx

import "strings"

// MaxUserAgentRunes is the CHECK (char_length(user_agent) <= 512) of sessions and audit_log (DD-38).
const MaxUserAgentRunes = 512

// NormalizeUserAgent makes a client-chosen User-Agent safe to store (DD-38, INV-33): each invalid
// UTF-8 byte becomes U+FFFD, U+0000 is dropped (PostgreSQL text cannot hold it) and only the first
// MaxUserAgentRunes runes are kept, never splitting a character and without adding "…". Without
// it, a long or malformed header made the INSERT fail and rolled back the audited operation.
// Both writers of the column call it: audit.Recorder.Record and identity's CreateSession.
func NormalizeUserAgent(raw string) string {
	var b strings.Builder
	runes := 0
	// ranging over a string yields utf8.RuneError (U+FFFD) for each invalid byte.
	for _, r := range raw {
		if r == 0 {
			continue
		}
		if runes == MaxUserAgentRunes {
			break
		}
		b.WriteRune(r)
		runes++
	}
	return b.String()
}
