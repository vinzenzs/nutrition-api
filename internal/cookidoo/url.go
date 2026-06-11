package cookidoo

import (
	"net/url"
	"regexp"
	"strings"
)

// hostPattern matches a Cookidoo host across locales: cookidoo.de, cookidoo.com,
// cookidoo.co.uk, cookidoo.ch, etc. The TLD is one or two letter-only labels, so
// cookidoo must be the registrable domain — this rejects label-extension tricks
// like cookidoo.de.evil.com where the real domain is evil.com.
var hostPattern = regexp.MustCompile(`^cookidoo\.[a-z]{2,}(\.[a-z]{2,})?$`)

// pathPattern matches the recipe path shape: /recipes/recipe/<locale>/<id>,
// optionally trailing-slashed. <locale> and <id> are single non-empty segments.
var pathPattern = regexp.MustCompile(`^/recipes/recipe/[^/]+/[^/]+/?$`)

// IsRecipeURL reports whether raw is a well-formed Cookidoo recipe URL. It is a
// pure check — no network — used to reject input before any outbound request.
func IsRecipeURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return false
	}
	if !hostPattern.MatchString(strings.ToLower(u.Hostname())) {
		return false
	}
	return pathPattern.MatchString(u.Path)
}

// ValidateRecipeURL returns ErrNotCookidooURL when raw is not a recognised
// Cookidoo recipe URL.
func ValidateRecipeURL(raw string) error {
	if !IsRecipeURL(raw) {
		return ErrNotCookidooURL
	}
	return nil
}
