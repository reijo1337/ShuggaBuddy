// Package i18n предоставляет инфраструктуру локализации для бота.
// Все пользовательские строки загружаются из YAML-файлов.
package i18n

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// Localizer управляет загрузкой и подстановкой локализованных строк.
type Localizer struct {
	mu       sync.RWMutex
	messages map[string]map[string]string // lang → key → value
	fallback string
}

// NewLocalizer создаёт Localizer с указанным языком по умолчанию
// и загружает соответствующий файл локализации из localesDir.
func NewLocalizer(localesDir, defaultLang string) (*Localizer, error) {
	l := &Localizer{
		messages: make(map[string]map[string]string),
		fallback: defaultLang,
	}

	if err := l.loadLang(localesDir, defaultLang); err != nil {
		return nil, fmt.Errorf("i18n.NewLocalizer: %w", err)
	}

	return l, nil
}

// loadLang загружает YAML-файл локализации для указанного языка.
func (l *Localizer) loadLang(localesDir, lang string) error {
	path := fmt.Sprintf("%s/%s.yaml", localesDir, lang)
	data, err := os.ReadFile(path) //nolint:gosec // path is built from trusted localesDir+lang, not user input
	if err != nil {
		return fmt.Errorf("loadLang(%s): %w", lang, err)
	}

	messages := make(map[string]string)
	if err := yaml.Unmarshal(data, &messages); err != nil {
		return fmt.Errorf("loadLang(%s): parse error: %w", lang, err)
	}

	l.mu.Lock()
	l.messages[lang] = messages
	l.mu.Unlock()

	return nil
}

// T возвращает локализованную строку по ключу для языка по умолчанию.
// Поддерживает подстановку аргументов через fmt.Sprintf.
// Если ключ не найден, возвращает сам ключ.
func (l *Localizer) T(key string, args ...any) string {
	return l.TLang(l.fallback, key, args...)
}

// TLang возвращает локализованную строку для конкретного языка.
// Если язык не загружен, используется fallback.
func (l *Localizer) TLang(lang, key string, args ...any) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	msgs, ok := l.messages[lang]
	if !ok {
		msgs = l.messages[l.fallback]
	}

	if msgs == nil {
		return key
	}

	tmpl, ok := msgs[key]
	if !ok {
		return key
	}

	if len(args) == 0 {
		return tmpl
	}

	return fmt.Sprintf(tmpl, args...)
}
