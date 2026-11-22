package main

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// Локализация: ключ - русская строка, как она записана в коде; для английского берётся перевод из enDict (i18n_en.go).
// Нет перевода - показывается русский оригинал. Выбор языка: Lang=auto|ru|en в crocaun.ini; auto = русский,
// если язык интерфейса Windows русский, иначе английский. Смена применяется после перезапуска.

var curLang atomic.Value // "ru" | "en"

func init() { curLang.Store(detectLang()); initUIStrings() }

// detectLang определяет язык системы.
func detectLang() string {
	if systemIsRussian() {
		return "ru"
	}
	return "en"
}

func systemIsRussian() bool {
	if v := os.Getenv("CROCAUN_LANG"); v != "" {
		return strings.HasPrefix(strings.ToLower(v), "ru")
	}
	return sysUILangRussian()
}

// normLang приводит значение настройки к auto|ru|en.
func normLang(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ru", "russian", "рус", "русский":
		return "ru"
	case "en", "english", "eng":
		return "en"
	}
	return "auto"
}

// setLang применяет выбранный язык ("auto" - по системе).
func setLang(pref string) {
	switch normLang(pref) {
	case "ru":
		curLang.Store("ru")
	case "en":
		curLang.Store("en")
	default:
		curLang.Store(detectLang())
	}
	initUIStrings()
}

func isEnglish() bool { return curLang.Load() == "en" }

// T переводит строку интерфейса.
func T(s string) string {
	if !isEnglish() {
		return s
	}
	if v, ok := enDict[s]; ok {
		return v
	}
	return s
}

// Tf - fmt.Sprintf с переводом формата.
func Tf(format string, a ...any) string { return fmt.Sprintf(T(format), a...) }
