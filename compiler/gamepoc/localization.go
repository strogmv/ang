package gamepoc

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// LocalizationCatalog is private world authoring. Story and undiscovered map
// translations must not be bundled with Unity's public interface vocabulary.
type LocalizationCatalog struct {
	DefaultLocale  string                `json:"default_locale"`
	FallbackLocale string                `json:"fallback_locale"`
	Locales        []string              `json:"locales"`
	Messages       []LocalizationMessage `json:"messages"`
}

type LocalizationMessage struct {
	Key     string            `json:"key"`
	Context string            `json:"context"`
	Texts   map[string]string `json:"texts"`
}

// ClientLocalization uses rows because Unity JsonUtility cannot decode maps.
// Context remains in authoring; only public ui.* texts enter these rows.
type ClientLocalization struct {
	DefaultLocale  string       `json:"default_locale"`
	FallbackLocale string       `json:"fallback_locale"`
	Locales        []string     `json:"locales"`
	Texts          []LocaleText `json:"texts"`
}

type LocaleText struct {
	Locale string `json:"locale"`
	Key    string `json:"key"`
	Text   string `json:"text"`
}

var localizationLocalePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
var localizationKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*(\.[a-z0-9_-]+)+$`)
var localizationArgumentPattern = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*|[0-9]+)$`)

func localizationArguments(text string) (string, error) {
	arguments := map[string]bool{}
	for i := 0; i < len(text); i++ {
		if text[i] != '{' && text[i] != '}' {
			continue
		}
		if i+1 < len(text) && text[i+1] == text[i] {
			i++
			continue
		}
		if text[i] == '}' {
			return "", fmt.Errorf("unmatched localization brace")
		}
		end := strings.IndexByte(text[i+1:], '}')
		if end < 0 {
			return "", fmt.Errorf("unmatched localization brace")
		}
		end += i + 1
		argument := text[i+1 : end]
		if split := strings.IndexAny(argument, ",:"); split >= 0 {
			argument = argument[:split]
		}
		argument = strings.TrimSpace(argument)
		if !localizationArgumentPattern.MatchString(argument) {
			return "", fmt.Errorf("invalid localization placeholder")
		}
		arguments[argument] = true
		i = end
	}
	keys := make([]string, 0, len(arguments))
	for key := range arguments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ","), nil
}

func (c LocalizationCatalog) Validate() error {
	// Earlier project fixtures have no catalog; their existing text remains a
	// legacy fallback. A partially configured catalog is never silently accepted.
	if c.DefaultLocale == "" && c.FallbackLocale == "" && len(c.Locales) == 0 && len(c.Messages) == 0 {
		return nil
	}
	if len(c.Locales) == 0 || len(c.Locales) > 32 || len(c.Messages) == 0 || len(c.Messages) > 65536 {
		return fmt.Errorf("invalid localization catalog size")
	}
	locales := map[string]bool{}
	for _, locale := range c.Locales {
		if !localizationLocalePattern.MatchString(locale) || locales[locale] {
			return fmt.Errorf("invalid/duplicate localization locale %q", locale)
		}
		locales[locale] = true
	}
	if !locales[c.DefaultLocale] || !locales[c.FallbackLocale] {
		return fmt.Errorf("localization default/fallback locale is not registered")
	}
	keys := map[string]bool{}
	for _, message := range c.Messages {
		if !localizationKeyPattern.MatchString(message.Key) || keys[message.Key] || strings.TrimSpace(message.Context) == "" || len(message.Texts) == 0 {
			return fmt.Errorf("invalid/duplicate localization message %q", message.Key)
		}
		keys[message.Key] = true
		arguments, first := "", true
		for locale, text := range message.Texts {
			if !locales[locale] || strings.TrimSpace(text) == "" {
				return fmt.Errorf("invalid localization translation %s/%s", message.Key, locale)
			}
			current, err := localizationArguments(text)
			if err != nil {
				return fmt.Errorf("%s/%s: %w", message.Key, locale, err)
			}
			if !first && current != arguments {
				return fmt.Errorf("localization placeholders differ for %s", message.Key)
			}
			arguments, first = current, false
		}
	}
	return nil
}

func clientLocalization(c LocalizationCatalog) ClientLocalization {
	result := ClientLocalization{
		DefaultLocale: c.DefaultLocale, FallbackLocale: c.FallbackLocale,
		Locales: append([]string(nil), c.Locales...), Texts: []LocaleText{},
	}
	for _, message := range c.Messages {
		if !strings.HasPrefix(message.Key, "ui.") {
			continue
		}
		for locale, text := range message.Texts {
			result.Texts = append(result.Texts, LocaleText{Locale: locale, Key: message.Key, Text: text})
		}
	}
	sort.Slice(result.Texts, func(i, j int) bool {
		a, b := result.Texts[i], result.Texts[j]
		if a.Locale != b.Locale {
			return a.Locale < b.Locale
		}
		return a.Key < b.Key
	})
	return result
}
