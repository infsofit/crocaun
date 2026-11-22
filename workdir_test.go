package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestParseWorkdirArg(t *testing.T) {
	cases := []struct {
		in   []string
		rest []string
		dir  string
		bad  bool
	}{
		{[]string{"-w", `C:\x`}, nil, `C:\x`, false},
		{[]string{"--workdir", "d", "--send", "a"}, []string{"--send", "a"}, "d", false},
		{[]string{"--workdir=q w"}, nil, "q w", false},
		{[]string{"-w=z", "--selftest"}, []string{"--selftest"}, "z", false},
		{[]string{"--guitest"}, []string{"--guitest"}, "", false},
		{[]string{"-w"}, nil, "", true},
		{[]string{"--workdir="}, nil, "", true},
	}
	for _, c := range cases {
		rest, dir, err := parseWorkdirArg(c.in)
		if (err != nil) != c.bad {
			t.Fatalf("%v: err=%v", c.in, err)
		}
		if c.bad {
			continue
		}
		if dir != c.dir || !reflect.DeepEqual(rest, c.rest) {
			t.Fatalf("%v: rest=%v dir=%q", c.in, rest, dir)
		}
	}
}

func TestExpandEnvPercent(t *testing.T) {
	t.Setenv("CROCAUN_T1", "abc")
	if got := expandEnvPercent(`%CROCAUN_T1%\x\%NO_SUCH_VAR_Q%`); got != `abc\x\%NO_SUCH_VAR_Q%` {
		t.Fatal(got)
	}
}

func TestSanitizeChatDir(t *testing.T) {
	for in, want := range map[string]string{
		"":          chatDirDefault,
		"  ":        chatDirDefault,
		"my chat":   "my chat",
		`a\b/c:d`:   "a_b_c_d",
		"con":       chatDirDefault,
		"..":        chatDirDefault,
		"имя папки": "имя папки",
		"name. ":    "name",
		"x*y?z":     "x_y_z",
	} {
		if got := sanitizeChatDir(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}

func writeAt(t *testing.T, p, content string, mt time.Time) {
	t.Helper()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateChatDir(t *testing.T) {
	base := t.TempDir()
	now := time.Now()
	// только старая папка: просто переименовывается
	writeAt(t, filepath.Join(base, chatDirLegacy, "a.log"), "old", now)
	migrateChatDir(base, "new1", chatDirLegacy)
	if b, err := os.ReadFile(filepath.Join(base, "new1", "a.log")); err != nil || string(b) != "old" {
		t.Fatalf("rename: %v %q", err, b)
	}
	if _, err := os.Stat(filepath.Join(base, chatDirLegacy)); err == nil {
		t.Fatal("старая папка осталась")
	}
	// слияние: побеждают новые по времени, остальные переносятся
	writeAt(t, filepath.Join(base, "src", "same-new.log"), "src-newer", now)
	writeAt(t, filepath.Join(base, "dst", "same-new.log"), "dst-older", now.Add(-time.Hour))
	writeAt(t, filepath.Join(base, "src", "same-old.log"), "src-older", now.Add(-time.Hour))
	writeAt(t, filepath.Join(base, "dst", "same-old.log"), "dst-newer", now)
	writeAt(t, filepath.Join(base, "src", "only-src.log"), "S", now)
	writeAt(t, filepath.Join(base, "dst", "only-dst.log"), "D", now)
	writeAt(t, filepath.Join(base, "src", "sub", "x.log"), "X", now)
	migrateChatDir(base, "dst", "src")
	want := map[string]string{"same-new.log": "src-newer", "same-old.log": "dst-newer", "only-src.log": "S", "only-dst.log": "D", filepath.Join("sub", "x.log"): "X"}
	for f, w := range want {
		b, err := os.ReadFile(filepath.Join(base, "dst", f))
		if err != nil || string(b) != w {
			t.Errorf("%s: %v %q (нужно %q)", f, err, b, w)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "src")); err == nil {
		t.Error("src не удалена")
	}
	// ничего нет - ничего не создаётся
	empty := t.TempDir()
	migrateChatDir(empty, "zzz", chatDirLegacy, "")
	if es, _ := os.ReadDir(empty); len(es) != 0 {
		t.Error("создана лишняя папка")
	}
}

func TestRoster(t *testing.T) {
	dir := t.TempDir()
	r, err := newChatRoom("t", "pw", dir)
	if err != nil {
		t.Fatal(err)
	}
	now := nowMs()
	r.mu.Lock()
	r.msgs = []ChatMsg{{ID: "1", UID: "me", TS: now - 5000, Kind: "msg"}, {ID: "2", UID: "u1", TS: now - 4000, Kind: "hi"}, {ID: "3", UID: "u2", TS: now - 3000, Kind: "msg"}}
	r.names["me"], r.names["u1"], r.names["u2"] = "Я", "Борис", "Аня"
	r.mu.Unlock()
	ro := r.Roster("me")
	if len(ro) != 3 || !ro[0].Self || ro[1].Name != "Аня" || ro[2].Name != "Борис" {
		t.Fatalf("%+v", ro)
	}
	if n := r.HideOffline("me", func(uid string) bool { return uid == "u2" }); n != 1 {
		t.Fatalf("убрано %d", n)
	}
	if ro = r.Roster("me"); len(ro) != 2 || ro[1].UID != "u2" {
		t.Fatalf("%+v", ro)
	}
	// вернётся после новой активности
	r.mu.Lock()
	r.msgs = append(r.msgs, ChatMsg{ID: "4", UID: "u1", TS: nowMs() + 10, Kind: "msg"})
	r.mu.Unlock()
	if ro = r.Roster("me"); len(ro) != 3 {
		t.Fatalf("%+v", ro)
	}
}

func TestLangNorm(t *testing.T) {
	for in, want := range map[string]string{"": "auto", "RU": "ru", "english": "en", "xx": "auto"} {
		if normLang(in) != want {
			t.Errorf("%q", in)
		}
	}
	setLang("en")
	if T("Параметр -w/--workdir:") != "Parameter -w/--workdir:" || T("нет в словаре") != "нет в словаре" {
		t.Error("T en")
	}
	setLang("ru")
	if T("Параметр -w/--workdir:") != "Параметр -w/--workdir:" {
		t.Error("T ru")
	}
	setLang("auto")
}
