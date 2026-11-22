package main

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

var fmtVerb = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z%]`)

// Перевод не должен терять или менять форматные подстановки и не должен быть пустым.
func TestEnDictSane(t *testing.T) {
	cyr := regexp.MustCompile(`[А-Яа-яЁё]`)
	for ru, en := range enDict {
		if strings.TrimSpace(en) == "" {
			t.Errorf("пустой перевод: %q", ru)
		}
		a, b := fmtVerb.FindAllString(ru, -1), fmtVerb.FindAllString(en, -1)
		sort.Strings(a)
		sort.Strings(b)
		if strings.Join(a, ",") != strings.Join(b, ",") {
			t.Errorf("подстановки не совпадают: %q -> %q", ru, en)
		}
		if strings.HasPrefix(ru, "\r\n") != strings.HasPrefix(en, "\r\n") || strings.HasSuffix(ru, "\r\n") != strings.HasSuffix(en, "\r\n") {
			t.Errorf("переводы строки по краям не совпадают: %q", ru)
		}
		if cyr.MatchString(en) && !strings.Contains(en, "Auto") {
			t.Errorf("в английском переводе остались русские буквы: %q", en)
		}
	}
}

func TestUIStringsFollowLang(t *testing.T) {
	setLang("en")
	if destInternet != "Over the internet" || trErr("отменено").Error() != "cancelled" {
		t.Errorf("en: %q", destInternet)
	}
	setLang("ru")
	if destInternet != "Через интернет" || trErr("отменено").Error() != "отменено" {
		t.Errorf("ru: %q", destInternet)
	}
	setLang("auto")
}
