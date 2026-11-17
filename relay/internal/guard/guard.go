// Package guard is the deterministic last check on what Roger is about to
// say out loud. It is a backstop, not the safety system: the persona prompt
// and model do the real work, and this catches the mechanical failures
// (runaway length, links, markup, a short list of words that must never be
// spoken to a child). See docs/SAFETY.md for what this does and does not do.
package guard

import (
	"regexp"
	"strings"
)

// MaxSpokenChars bounds a reply to roughly 45 seconds of speech.
const MaxSpokenChars = 700

// Fallback is spoken when a reply is rejected outright.
const Fallback = "Hmm, I'm not sure about that one. That's a good question for a grown-up. Over."

var (
	urlRe    = regexp.MustCompile(`(?i)\b(?:https?://|www\.)\S+`)
	markupRe = regexp.MustCompile("[*_#`>|~]+")
	spaceRe  = regexp.MustCompile(`\s+`)
	// Deliberately short. A long blocklist gives false confidence; these are
	// words with no legitimate use in a reply to a 4-12 year old.
	blockRe = regexp.MustCompile(`(?i)\b(fuck\w*|shit\w*|bitch\w*|asshole\w*|cunt\w*|porn\w*|nigg\w*|fagg\w*)\b`)
)

// Result describes what the guard did.
type Result struct {
	Text      string
	Blocked   bool
	Truncated bool
}

// Check cleans a reply for speech and rejects it if it trips the blocklist.
func Check(say string) Result {
	s := urlRe.ReplaceAllString(say, "")
	s = markupRe.ReplaceAllString(s, "")
	s = strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
	if s == "" || blockRe.MatchString(s) {
		return Result{Text: Fallback, Blocked: true}
	}
	if len(s) <= MaxSpokenChars {
		return Result{Text: s}
	}
	cut := s[:MaxSpokenChars]
	if i := strings.LastIndexAny(cut, ".!?"); i > MaxSpokenChars/2 {
		cut = cut[:i+1]
	} else if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i] + "."
	}
	return Result{Text: cut, Truncated: true}
}
