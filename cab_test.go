package main

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// readCABForTest - независимый разбор CAB/MSZIP: проверяет структуру, контрольные суммы блоков и содержимое.
func readCABForTest(b []byte) (map[string][]byte, []string, error) {
	le := binary.LittleEndian
	if len(b) < 44 || string(b[:4]) != "MSCF" {
		return nil, nil, fmt.Errorf("нет сигнатуры MSCF")
	}
	if int(le.Uint32(b[8:])) != len(b) {
		return nil, nil, fmt.Errorf("cbCabinet=%d, реально %d", le.Uint32(b[8:]), len(b))
	}
	coffFiles := int(le.Uint32(b[16:]))
	nFolders, nFiles := int(le.Uint16(b[26:])), int(le.Uint16(b[28:]))
	if nFolders != 1 || b[24] != 3 || b[25] != 1 {
		return nil, nil, fmt.Errorf("неожиданная версия/число папок")
	}
	coffData, nData, comp := int(le.Uint32(b[36:])), int(le.Uint16(b[40:])), le.Uint16(b[42:])
	if comp != 1 {
		return nil, nil, fmt.Errorf("не MSZIP")
	}
	type ent struct {
		name      string
		size, off int
	}
	var ents []ent
	p := coffFiles
	for i := 0; i < nFiles; i++ {
		size, off := int(le.Uint32(b[p:])), int(le.Uint32(b[p+4:]))
		p += 16
		e := bytes.IndexByte(b[p:], 0)
		ents = append(ents, ent{string(b[p : p+e]), size, off})
		p += e + 1
	}
	if p != coffData {
		return nil, nil, fmt.Errorf("coffCabStart=%d, ожидалось %d", coffData, p)
	}
	var stream []byte
	p = coffData
	for i := 0; i < nData; i++ {
		csum, cb, cu := le.Uint32(b[p:]), int(le.Uint16(b[p+4:])), int(le.Uint16(b[p+6:]))
		data := b[p+8 : p+8+cb]
		if want := cabChecksum(data, cabChecksum(b[p+4:p+8], 0)); csum != want {
			return nil, nil, fmt.Errorf("блок %d: csum %08x, ожидалось %08x", i, csum, want)
		}
		if string(data[:2]) != "CK" {
			return nil, nil, fmt.Errorf("блок %d: нет CK", i)
		}
		var out bytes.Buffer
		r := flate.NewReader(bytes.NewReader(data[2:]))
		if _, err := out.ReadFrom(r); err != nil {
			return nil, nil, fmt.Errorf("блок %d: %v", i, err)
		}
		if out.Len() != cu || (i < nData-1 && cu != cabBlockSize) {
			return nil, nil, fmt.Errorf("блок %d: размер %d, ожидалось %d", i, out.Len(), cu)
		}
		stream = append(stream, out.Bytes()...)
		p += 8 + cb
	}
	if p != len(b) {
		return nil, nil, fmt.Errorf("лишние данные в конце CAB")
	}
	res := map[string][]byte{}
	var order []string
	for _, e := range ents {
		if e.off+e.size > len(stream) {
			return nil, nil, fmt.Errorf("файл %s выходит за поток", e.name)
		}
		res[e.name] = stream[e.off : e.off+e.size]
		order = append(order, e.name)
	}
	return res, order, nil
}

func rnd(n int) []byte { b := make([]byte, n); _, _ = rand.Read(b); return b }

func TestCABRoundTrip(t *testing.T) {
	files := []cabFile{
		{"empty.dat", nil, time.Now()},
		{"a.txt", []byte("hello"), time.Now()},
		{`dir\sub\rnd.bin`, rnd(100000), time.Now()},
		{"zeros.bin", make([]byte, 70000), time.Now()},
		{"exact.bin", rnd(cabBlockSize * 2), time.Now()},
		{"tail1.bin", rnd(1), time.Now()},
		{"tail3.bin", rnd(3), time.Now()},
	}
	cab, err := buildCAB(files)
	if err != nil {
		t.Fatal(err)
	}
	got, order, err := readCABForTest(cab)
	if err != nil {
		t.Fatal(err)
	}
	for i, f := range files {
		if order[i] != f.Name || !bytes.Equal(got[f.Name], f.Data) {
			t.Errorf("файл %s не совпал", f.Name)
		}
	}
}

func TestBatScript(t *testing.T) {
	dir := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(dir, "папка", "Вложенная"), 0755))
	data1, data2 := rnd(50000), []byte("текст в файле")
	must(os.WriteFile(filepath.Join(dir, "папка", "Вложенная", "Файл №1.bin"), data1, 0644))
	must(os.WriteFile(filepath.Join(dir, "папка", "readme.txt"), data2, 0644))
	exe := filepath.Join(dir, "fake.exe")
	exeData := rnd(200000)
	must(os.WriteFile(exe, exeData, 0644))

	res, err := buildBat(BatOptions{Text: "Привет!\nвторая строка", Items: []string{filepath.Join(dir, "папка")}, EmbedExe: true, ExePath: exe, PartSize: 2000})
	if err != nil {
		t.Fatal(err)
	}
	s := res.Script
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			t.Fatalf("не-ASCII символ в скрипте на позиции %d", i)
		}
	}
	if strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") || strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\r") {
		t.Fatal("есть строки без CRLF")
	}
	lines := strings.Split(strings.TrimSuffix(s, "\r\n"), "\r\n")
	marker := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "#PAYLOAD") {
			if marker >= 0 {
				t.Fatal("маркер встречается дважды в начале строки")
			}
			marker = i
		}
	}
	if marker < 0 {
		t.Fatal("нет маркера")
	}
	payload := lines[marker+1:]
	if len(payload) != res.PayloadLines || !strings.Contains(s, fmt.Sprintf(`set "CN_L=%d"`, len(payload))) {
		t.Fatalf("CN_L не совпал: %d строк", len(payload))
	}
	for _, l := range payload {
		if len(l) == 0 || len(l) > batLineLen || strings.Trim(l, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=") != "" {
			t.Fatalf("плохая строка payload: %q", l)
		}
	}
	// части
	if strings.Join(res.Parts, "") != s {
		t.Fatal("части не склеиваются в исходный скрипт")
	}
	for i, p := range res.Parts {
		if len(p) > 2000 {
			t.Fatalf("часть %d длиннее лимита: %d", i, len(p))
		}
	}
	if len(res.Parts) < 2 {
		t.Fatal("ожидалось несколько частей")
	}
	// содержимое
	raw, err := base64.StdEncoding.DecodeString(strings.Join(payload, ""))
	if err != nil {
		t.Fatal(err)
	}
	got, order, err := readCABForTest(raw)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]byte{
		batMessageName:                  []byte("Привет!\r\nвторая строка"),
		`papka\Vlozhennaya\Fajl _1.bin`: data1,
		`papka\readme.txt`:              data2,
		batExeName:                      exeData,
	}
	t.Logf("файлы в CAB: %q", order)
	for n, d := range want {
		if !bytes.Equal(got[n], d) {
			t.Errorf("нет или не совпал: %q", n)
		}
	}
	// слишком короткая часть
	if _, err := buildBat(BatOptions{Text: "x", PartSize: 100}); err == nil {
		t.Error("ожидалась ошибка для PartSize=100")
	}
}

// Все строки шапки обязаны начинаться НЕ с символа base64: скрипт выбирает данные через findstr /b /r "[A-Za-z0-9+/=]".
func TestBatPayloadFilter(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "x.exe")
	if err := os.WriteFile(exe, rnd(5000), 0644); err != nil {
		t.Fatal(err)
	}
	for _, o := range []BatOptions{{Text: "hi"}, {Text: "hi", EmbedExe: true, ExePath: exe}, {EmbedExe: true, ExePath: exe, PartSize: 700}} {
		res, err := buildBat(o)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, l := range strings.Split(strings.TrimSuffix(res.Script, "\r\n"), "\r\n") {
			if l != "" && strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=", rune(l[0])) {
				n++
			}
		}
		if n != res.PayloadLines {
			t.Errorf("%+v: фильтр выберет %d строк, а данных %d", o.EmbedExe, n, res.PayloadLines)
		}
	}
}
