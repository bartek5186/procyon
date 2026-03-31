package middleware

import (
	"sort"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type KeyLang struct{}
type KeyLangCandidates struct{}

type langPref struct {
	tag   string
	q     float64
	order int
}

func LanguageMiddleware(defaultLang string, available []string) echo.MiddlewareFunc {
	defaultLang = normalizeLang(defaultLang)
	defaultBase := baseLang(defaultLang)

	allowed := make(map[string]struct{}, len(available)+2)
	for _, a := range available {
		a = normalizeLang(a)
		if a == "" {
			continue
		}
		allowed[a] = struct{}{}
	}

	if _, ok := allowed[defaultLang]; !ok && defaultLang != "" {
		allowed[defaultLang] = struct{}{}
	}
	if _, ok := allowed[defaultBase]; !ok && defaultBase != "" {
		allowed[defaultBase] = struct{}{}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Accept-Language")

			preferred := parseAcceptLanguage(header)
			if len(preferred) == 0 {
				preferred = []string{defaultLang}
			}

			candidates := make([]string, 0, 8)
			seen := map[string]struct{}{}

			addIfAllowed := func(s string) {
				s = normalizeLang(s)
				if s == "" || s == "*" {
					return
				}
				if _, ok := allowed[s]; !ok {
					return
				}
				if _, ok := seen[s]; ok {
					return
				}
				seen[s] = struct{}{}
				candidates = append(candidates, s)
			}

			for _, tag := range preferred {
				addIfAllowed(tag)
				addIfAllowed(baseLang(tag))
			}

			addIfAllowed(defaultLang)
			addIfAllowed(defaultBase)

			resolved := defaultLang
			if len(candidates) > 0 {
				resolved = candidates[0]
			} else {
				candidates = []string{defaultLang}
			}

			c.Set("lang", resolved)
			c.Set("langBase", baseLang(resolved))
			c.Set("langCandidates", candidates)

			return next(c)
		}
	}
}

func normalizeLang(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "_", "-")
	return strings.ToLower(s)
}

func baseLang(tag string) string {
	tag = normalizeLang(tag)
	if tag == "" {
		return ""
	}
	if i := strings.IndexByte(tag, '-'); i != -1 {
		return tag[:i]
	}
	return tag
}

func parseAcceptLanguage(header string) []string {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil
	}

	parts := strings.Split(header, ",")
	prefs := make([]langPref, 0, len(parts))

	for i, raw := range parts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		seg := strings.Split(raw, ";")
		tag := normalizeLang(seg[0])
		if tag == "" {
			continue
		}

		q := 1.0
		for _, p := range seg[1:] {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "q=") {
				if v, err := strconv.ParseFloat(strings.TrimPrefix(p, "q="), 64); err == nil {
					q = v
				}
			}
		}

		if q <= 0 {
			continue
		}

		prefs = append(prefs, langPref{tag: tag, q: q, order: i})
	}

	if len(prefs) == 0 {
		return nil
	}

	sort.SliceStable(prefs, func(i, j int) bool {
		if prefs[i].q == prefs[j].q {
			return prefs[i].order < prefs[j].order
		}
		return prefs[i].q > prefs[j].q
	})

	out := make([]string, 0, len(prefs))
	seen := map[string]struct{}{}

	for _, p := range prefs {
		if _, ok := seen[p.tag]; ok {
			continue
		}
		seen[p.tag] = struct{}{}
		out = append(out, p.tag)
	}

	return out
}
