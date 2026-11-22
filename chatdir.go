package main

import (
	"os"
	"path/filepath"
	"strings"
)

// Папка чата: по умолчанию crocaun-chat (раньше - crocau-chat); имя можно сменить в настройках.
// Создаётся только когда чат действительно включён.

const (
	chatDirDefault = "crocaun-chat"
	chatDirLegacy  = "crocau-chat"
)

// chatDirName - текущее имя папки чата (из настроек).
var chatDirName = chatDirDefault

// sanitizeChatDir делает из введённого текста допустимое имя одной папки; пусто/плохо - значение по умолчанию.
func sanitizeChatDir(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 32, strings.ContainsRune(`\/:*?"<>|`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	s = strings.Trim(b.String(), ". ")
	if len(s) > 60 {
		s = strings.TrimRight(string([]rune(s)[:40]), ". ")
	}
	switch strings.ToUpper(s) {
	case "", "CON", "PRN", "AUX", "NUL", "COM1", "COM2", "COM3", "LPT1", "LPT2", "LPT3":
		return chatDirDefault
	}
	return s
}

// mergeDirNewest переносит содержимое каталога src в dst. Одинаковые имена файлов: остаётся более новый
// (по времени изменения), старый удаляется. src после переноса удаляется. Возвращает число конфликтов.
func mergeDirNewest(src, dst string) (conflicts int, err error) {
	if err = os.MkdirAll(dst, 0o700); err != nil {
		return
	}
	ents, err := os.ReadDir(src)
	if err != nil {
		return
	}
	for _, e := range ents {
		sp, dp := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		if e.IsDir() {
			c, _ := mergeDirNewest(sp, dp)
			conflicts += c
			continue
		}
		di, derr := os.Stat(dp)
		if derr != nil { // в целевой папке такого нет: просто переносим
			if os.Rename(sp, dp) != nil {
				if b, rerr := os.ReadFile(sp); rerr == nil && os.WriteFile(dp, b, 0o600) == nil {
					_ = os.Remove(sp)
				}
			}
			continue
		}
		conflicts++
		si, serr := os.Stat(sp)
		if serr == nil && si.ModTime().After(di.ModTime()) { // источник новее: заменяем
			_ = os.Remove(dp)
			if os.Rename(sp, dp) != nil {
				if b, rerr := os.ReadFile(sp); rerr == nil && os.WriteFile(dp, b, 0o600) == nil {
					_ = os.Remove(sp)
				}
			}
		} else {
			_ = os.Remove(sp)
		}
	}
	_ = os.Remove(src) // удалится, только если опустел
	return
}

// migrateChatDir приводит папки чата к выбранному имени: legacy и ранее использованное имя (prev) переименовываются
// или сливаются с целевой. Ничего не создаёт, если старых папок нет.
func migrateChatDir(base, want string, olds ...string) {
	dst := filepath.Join(base, want)
	for _, o := range olds {
		if o == "" || strings.EqualFold(o, want) {
			continue
		}
		src := filepath.Join(base, o)
		if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
			continue
		}
		if _, err := os.Stat(dst); err != nil {
			if os.Rename(src, dst) == nil {
				continue
			}
		}
		_, _ = mergeDirNewest(src, dst)
	}
}
