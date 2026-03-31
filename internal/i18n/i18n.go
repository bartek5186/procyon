package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu           sync.RWMutex
	translations = map[string]map[string]string{}
)

func LoadTranslations() error {
	files, err := filepath.Glob("internal/i18n/messages_*.json")
	if err != nil {
		return err
	}

	loaded := make(map[string]map[string]string, len(files))

	for _, file := range files {
		base := filepath.Base(file)
		lang := strings.TrimSuffix(strings.TrimPrefix(base, "messages_"), ".json")
		if lang == "" {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", file, err)
		}

		var messages map[string]string
		if err := json.Unmarshal(content, &messages); err != nil {
			return fmt.Errorf("parse %s: %w", file, err)
		}

		loaded[normalizeLang(lang)] = messages
	}

	mu.Lock()
	translations = loaded
	mu.Unlock()

	return nil
}

func T(lang, key string, vars map[string]string) string {
	mu.RLock()
	defer mu.RUnlock()

	lang = normalizeLang(lang)
	base := baseLang(lang)

	if msg, ok := lookup(lang, key); ok {
		return interpolate(msg, vars)
	}
	if msg, ok := lookup(base, key); ok {
		return interpolate(msg, vars)
	}
	if msg, ok := lookup("pl", key); ok {
		return interpolate(msg, vars)
	}

	return key
}

func lookup(lang, key string) (string, bool) {
	if lang == "" {
		return "", false
	}

	msgs, ok := translations[lang]
	if !ok {
		return "", false
	}

	msg, ok := msgs[key]
	return msg, ok
}

func interpolate(input string, vars map[string]string) string {
	out := input
	for key, value := range vars {
		out = strings.ReplaceAll(out, "{{"+key+"}}", value)
	}
	return out
}

func normalizeLang(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "_", "-")
	return s
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
