package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// batTest (--battest, для CI на Windows): генерирует самораспаковывающиеся скрипты и запускает их настоящим cmd.exe
// с принудительным выбором декодера (certutil / VBScript / PowerShell). Проверяет папки, кириллицу, обрезанную вставку,
// склейку частей и встроенный crocaun.exe.
func batTest() int {
	if runtime.GOOS != "windows" {
		testLogf("BATTEST: только для Windows, пропуск")
		return 0
	}
	fails := 0
	check := func(name string, ok bool, detail string) {
		if ok {
			testLogf("BAT %s: OK %s", name, detail)
		} else {
			fails++
			testLogf("BAT %s: FAIL %s", name, detail)
		}
	}
	root, err := os.MkdirTemp("", "battest-")
	if err != nil {
		testLogf("BATTEST: нет временной папки: %v", err)
		return 1
	}
	defer os.RemoveAll(root)

	// runBat запускает скрипт в новой пустой папке и возвращает её путь, вывод, код возврата и время.
	runN := 0
	runBat := func(script, dec string) (dir, out string, code int, dur time.Duration) {
		runN++
		dir = filepath.Join(root, "run"+strconv.Itoa(runN))
		_ = os.MkdirAll(dir, 0755)
		if err := os.WriteFile(filepath.Join(dir, "test.bat"), []byte(script), 0644); err != nil {
			return dir, err.Error(), -1, 0
		}
		testLogf("BAT RUN #%d dec=%q: старт", runN, dec)
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "cmd.exe", "/c", "test.bat")
		cmd.Dir = dir
		// при зависании: снимок процессов (видно, какая команда ждёт) и принудительное завершение всего дерева
		cmd.WaitDelay = 10 * time.Second
		cmd.Cancel = func() error {
			if o, err := exec.Command("tasklist", "/FO", "CSV", "/NH").Output(); err == nil {
				var hung []string
				for _, l := range strings.Split(string(o), "\n") {
					low := strings.ToLower(l)
					for _, n := range []string{"cmd.exe", "more.com", "certutil", "cscript", "powershell", "expand", "find.exe", "findstr", "chcp"} {
						if strings.Contains(low, n) {
							hung = append(hung, strings.TrimSpace(l))
							break
						}
					}
				}
				testLogf("BAT RUN #%d: ТАЙМАУТ, процессы: %s", runN, strings.Join(hung, " ; "))
			}
			if cmd.Process != nil {
				_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
				return cmd.Process.Kill()
			}
			return nil
		}
		var env []string
		for _, e := range os.Environ() {
			if !strings.HasPrefix(strings.ToUpper(e), "CROCAUN_") {
				env = append(env, e)
			}
		}
		env = append(env, "CROCAUN_NOPAUSE=1")
		if dec != "" {
			env = append(env, "CROCAUN_DEC="+dec)
		}
		cmd.Env = env
		hideWindow(cmd)
		var buf bytes.Buffer
		cmd.Stdout, cmd.Stderr = &buf, &buf
		t0 := time.Now()
		err := cmd.Run()
		dur = time.Since(t0)
		out = buf.String()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				code = -1
				out += " [" + err.Error() + "]"
			}
		}
		return
	}
	short := func(s string) string {
		s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", " | "))
		if len(s) > 700 {
			s = s[:700] + "..."
		}
		return s
	}

	// смоук: сама обвязка запуска cmd.exe работает и не зависает
	{
		dir, out, code, dur := runBat("@echo off\r\necho harness-ok\r\nexit /b 0\r\n", "")
		check("HARNESS", code == 0 && strings.Contains(out, "harness-ok"), "("+dur.Round(10*time.Millisecond).String()+") "+dir[len(dir)-5:]+" "+short(out))
	}

	cabExperiment(root)

	// исходное дерево
	src := filepath.Join(root, "src")
	_ = os.MkdirAll(filepath.Join(src, "дир", "вложенная"), 0755)
	_ = os.MkdirAll(filepath.Join(src, "sub", "deep"), 0755)
	bin1, big := randBytes(120*1024), randBytes(700*1024)
	files := map[string][]byte{
		`src\readme.txt`:               []byte("hello"),
		`src\dir\vlozhennaya\fajl.bin`: bin1,
		`src\sub\deep\a.txt`:           []byte("deep file\r\nline2"),
		`src\empty.dat`:                {},
		`src\big.bin`:                  big,
	}
	_ = os.WriteFile(filepath.Join(src, "readme.txt"), files[`src\readme.txt`], 0644)
	_ = os.WriteFile(filepath.Join(src, "дир", "вложенная", "файл.bin"), bin1, 0644)
	_ = os.WriteFile(filepath.Join(src, "sub", "deep", "a.txt"), files[`src\sub\deep\a.txt`], 0644)
	_ = os.WriteFile(filepath.Join(src, "empty.dat"), nil, 0644)
	_ = os.WriteFile(filepath.Join(src, "big.bin"), big, 0644)
	msg := "Привет! Это сообщение.\nВторая строка."

	res, err := buildBat(BatOptions{Text: msg, Items: []string{src}})
	if err != nil {
		testLogf("BATTEST: buildBat: %v", err)
		return 1
	}
	testLogf("BAT INFO: скрипт %d символов, CAB %d байт, строк base64 %d", len(res.Script), res.CabBytes, res.PayloadLines)

	verify := func(dir string) (bool, string) {
		for rel, want := range files {
			got, err := os.ReadFile(filepath.Join(dir, rel))
			if err != nil || !bytes.Equal(got, want) {
				return false, "не совпал " + rel
			}
		}
		m, err := os.ReadFile(filepath.Join(dir, batMessageName))
		if err != nil || string(m) != toCRLF(msg) {
			return false, "сообщение не совпало"
		}
		return true, ""
	}

	// 1. каждый декодер отдельно + выбор по умолчанию
	for _, dec := range []string{"", "cert", "vbs", "ps"} {
		name := dec
		if name == "" {
			name = "auto"
		}
		dir, out, code, dur := runBat(res.Script, dec)
		ok, why := verify(dir)
		check("DECODER "+name, code == 0 && ok, "("+dur.Round(10*time.Millisecond).String()+") "+why+" "+short(out))
	}

	// 2. обрезанная вставка должна распознаваться
	lines := strings.Split(strings.TrimSuffix(res.Script, "\r\n"), "\r\n")
	cut := strings.Join(lines[:len(lines)-3], "\r\n") + "\r\n"
	_, out, code, _ := runBat(cut, "cert")
	check("TRUNCATED", code != 0 && strings.Contains(out, "incomplete"), "code="+strconv.Itoa(code)+" "+short(out))

	// 3. части склеиваются и работают
	resP, err := buildBat(BatOptions{Text: msg, Items: []string{src}, PartSize: 1500})
	if err != nil {
		check("PARTS build", false, err.Error())
	} else {
		okLen := len(resP.Parts) > 1
		for _, p := range resP.Parts {
			if len(p) > 1500 {
				okLen = false
			}
		}
		dir, out, code, _ := runBat(strings.Join(resP.Parts, ""), "cert")
		ok, why := verify(dir)
		check("PARTS", okLen && code == 0 && ok, "частей: "+strconv.Itoa(len(resP.Parts))+" "+why+" "+short(out))
	}

	// 4. справка: что делает expand.exe с именами в UTF-8 (не влияет на результат теста)
	if resU, err := buildBat(BatOptions{Text: "u", Items: []string{src}, UTF8Names: true}); err == nil {
		dir, out, code, _ := runBat(resU.Script, "cert")
		var names []string
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && d.Name() != "test.bat" {
				r, _ := filepath.Rel(dir, p)
				names = append(names, r)
			}
			return nil
		})
		testLogf("BAT INFO utf8-names: code=%d files=%q out=%s", code, names, short(out))
	}

	// 5. встроенный crocaun.exe (реальный размер: проверяем большие данные каждым декодером)
	exe, _ := os.Executable()
	exeData, _ := os.ReadFile(exe)
	wantSum := sha256.Sum256(exeData)
	t0 := time.Now()
	resE, err := buildBat(BatOptions{Text: "exe inside", EmbedExe: true})
	if err != nil {
		check("EMBED build", false, err.Error())
	} else {
		testLogf("BAT INFO embed: exe %d байт -> скрипт %d символов (%d строк), сборка %s",
			len(exeData), len(resE.Script), resE.PayloadLines, time.Since(t0).Round(10*time.Millisecond))
		for _, dec := range []string{"cert", "vbs", "ps"} {
			dir, out, code, dur := runBat(resE.Script, dec)
			got, rerr := os.ReadFile(filepath.Join(dir, batExeName))
			ok := rerr == nil && sha256.Sum256(got) == wantSum
			check("EMBED "+dec, code == 0 && ok, "("+dur.Round(10*time.Millisecond).String()+") "+short(out))
		}
	}

	if fails > 0 {
		testLogf("BATTEST FAILED: %d", fails)
		return 1
	}
	testLogf("BATTEST PASSED")
	return 0
}

// cabExperiment (временная диагностика): какие свойства набора файлов ломают expand.exe (0xca00a009).
func cabExperiment(root string) {
	dir := filepath.Join(root, "cabx")
	_ = os.MkdirAll(dir, 0755)
	now := time.Now()
	text := func(n int) []byte {
		var b []byte
		for len(b) < n {
			b = append(b, []byte("The quick brown fox jumps over the lazy dog 0123456789\r\n")...)
		}
		return b[:n]
	}
	small := cabFile{"small.txt", []byte("small"), now}
	type v struct {
		name  string
		files []cabFile
		o     cabOpts
	}
	for i, c := range []v{
		{"random700k+small", []cabFile{{"r.bin", randBytes(700 * 1024), now}, small}, cabOpts{}},
		{"random700k+small STORED", []cabFile{{"r.bin", randBytes(700 * 1024), now}, small}, cabOpts{stored: true}},
		{"random100k+small", []cabFile{{"r.bin", randBytes(100 * 1024), now}, small}, cabOpts{}},
		{"random33k+small", []cabFile{{"r.bin", randBytes(33 * 1024), now}, small}, cabOpts{}},
		{"text700k+small", []cabFile{{"t.txt", text(700 * 1024), now}, small}, cabOpts{}},
		{"small+EMPTY+small", []cabFile{small, {"e.dat", nil, now}, {"s2.txt", []byte("two"), now}}, cabOpts{}},
		{"empty last", []cabFile{small, {"s2.txt", []byte("two"), now}, {"e.dat", nil, now}}, cabOpts{}},
		{"empty first", []cabFile{{"e.dat", nil, now}, small, {"s2.txt", []byte("two"), now}}, cabOpts{}},
	} {
		b, err := buildCABOpts(c.files, c.o)
		if err != nil {
			testLogf("BAT CABX %s: сборка: %v", c.name, err)
			continue
		}
		cab := filepath.Join(dir, "t"+strconv.Itoa(i)+".cab")
		out := filepath.Join(dir, "o"+strconv.Itoa(i))
		_ = os.WriteFile(cab, b, 0644)
		_ = os.MkdirAll(out, 0755)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		cmd := exec.CommandContext(ctx, "expand.exe", cab, "-F:*", out)
		cmd.WaitDelay = 5 * time.Second
		hideWindow(cmd)
		var buf bytes.Buffer
		cmd.Stdout, cmd.Stderr = &buf, &buf
		err = cmd.Run()
		cancel()
		code := 0
		if err != nil {
			code = -1
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			}
		}
		okAll := true
		for _, f := range c.files {
			got, rerr := os.ReadFile(filepath.Join(out, f.Name))
			if rerr != nil || !bytes.Equal(got, f.Data) {
				okAll = false
			}
		}
		o := buf.String()
		if j := strings.Index(o, "Error Code"); j >= 0 {
			o = o[j:]
		} else {
			o = ""
		}
		if len(o) > 80 {
			o = o[:80]
		}
		testLogf("BAT CABX %-26s cab=%7d code=%d совпало=%v %s", c.name, len(b), code, okAll, strings.TrimSpace(strings.ReplaceAll(o, "\r\n", " ")))
	}
}
