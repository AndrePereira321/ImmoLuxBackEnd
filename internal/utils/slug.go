package utils

import (
	"regexp"
	"strings"
)

var slugAccentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "õ", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
	"Á", "a", "À", "a", "Â", "a", "Ã", "a", "Ä", "a",
	"É", "e", "È", "e", "Ê", "e", "Ë", "e",
	"Í", "i", "Ì", "i", "Î", "i", "Ï", "i",
	"Ó", "o", "Ò", "o", "Ô", "o", "Õ", "o", "Ö", "o",
	"Ú", "u", "Ù", "u", "Û", "u", "Ü", "u",
	"Ç", "c", "Ñ", "n",
)

var slugNonAlphanumRe = regexp.MustCompile(`[^a-z0-9-]+`)
var slugMultiHyphenRe = regexp.MustCompile(`-{2,}`)

// Slugify converts a location name to a URL-safe slug.
// "Viana do Castelo" → "viana-do-castelo"
// "São Brás de Alportel" → "sao-bras-de-alportel"
func Slugify(name string) string {
	if name == "" {
		return ""
	}
	s := strings.ToLower(name)
	s = slugAccentReplacer.Replace(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = slugNonAlphanumRe.ReplaceAllString(s, "")
	s = slugMultiHyphenRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
