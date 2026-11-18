package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Самораспаковывающийся BAT: файлы и текст -> CAB (MSZIP) -> base64 -> текст, который можно отправить
// в любом мессенджере. Получатель вставляет текст в свой .bat и запускает.
//
// Ничего, кроме cmd.exe и expand.exe, получателю не нужно. Base64 декодируется по цепочке:
// certutil -> VBScript (MSXML + ADODB.Stream) -> PowerShell. .NET не требуется нигде.

const (
	batMessageName = "crocaun-message.txt"
	batExeName     = "crocaun.exe"
	batLineLen     = 76 // длина строки base64, как в MIME
	batMaxTotal    = 256 << 20
	batMinPart     = 600
)

type BatOptions struct {
	Text      string
	Items     []string
	EmbedExe  bool   // положить crocaun.exe внутрь скрипта
	ExePath   string // для тестов; по умолчанию - сама программа
	PartSize  int    // макс. символов в одной части (0 - не делить)
	UTF8Names bool   // оставить не-ASCII имена как есть (по умолчанию транслитерация)
}

type BatResult struct {
	Script       string   // скрипт целиком
	Parts        []string // он же, разбитый на части (одна часть, если не делили)
	FileCount    int      // файлов пользователя (без служебных)
	CabBytes     int      // размер CAB
	PayloadLines int      // строк base64
	Notes        []string // замечания для пользователя
}

// Шапка скрипта. Только ASCII: получатель может сохранить файл в любой кодировке.
// Первая строка - @rem: если Блокнот добавит BOM, испортится только она.
// Декодер можно принудительно выбрать переменной CROCAUN_DEC=cert|vbs|ps (для тестов), CROCAUN_NOPAUSE отключает pause.
const batHeader = `@rem crocaun self-extracting script. Paste ALL parts, in order, into one .bat file and run it.
@echo off
%~d0
cd /D "%~p0"
setlocal
set "CN_F=%~f0"
set "CN_L=@LINES@"
set "CN_T=%TEMP%\cn%RANDOM%%RANDOM%"
md "%CN_T%" 2>nul
if not exist "%CN_T%\" set "CN_T=%CD%\cn%RANDOM%"
md "%CN_T%" 2>nul
set "CN_N="
for /f "delims=:" %%n in ('findstr /n /b /c:"#PAYLOAD" "%CN_F%"') do set "CN_N=%%n"
if not defined CN_N goto cn_bad
more +%CN_N% "%CN_F%" > "%CN_T%\p.b64"
set "CN_G=0"
for /f %%c in ('type "%CN_T%\p.b64" ^| find /c /v ""') do set "CN_G=%%c"
if %CN_G% LSS %CN_L% goto cn_short
set "CN_C=%CN_T%\a.cab"
if /i "%CROCAUN_DEC%"=="vbs" goto cn_vbs
if /i "%CROCAUN_DEC%"=="ps" goto cn_ps
certutil -f -decode "%CN_T%\p.b64" "%CN_C%" >nul 2>&1
if exist "%CN_C%" for %%a in ("%CN_C%") do if %%~za GTR 0 goto cn_ok
del "%CN_C%" 2>nul
if /i "%CROCAUN_DEC%"=="cert" goto cn_fail
:cn_vbs
del "%CN_T%\d.vbs" 2>nul
>>"%CN_T%\d.vbs" echo Set x=CreateObject("MSXML2.DOMDocument")
>>"%CN_T%\d.vbs" echo Set e=x.createElement("b")
>>"%CN_T%\d.vbs" echo e.dataType="bin.base64"
>>"%CN_T%\d.vbs" echo Set f=CreateObject("Scripting.FileSystemObject")
>>"%CN_T%\d.vbs" echo Set t=f.OpenTextFile(WScript.Arguments(0),1)
>>"%CN_T%\d.vbs" echo e.text=t.ReadAll
>>"%CN_T%\d.vbs" echo t.Close
>>"%CN_T%\d.vbs" echo Set s=CreateObject("ADODB.Stream")
>>"%CN_T%\d.vbs" echo s.Type=1
>>"%CN_T%\d.vbs" echo s.Open
>>"%CN_T%\d.vbs" echo s.Write e.nodeTypedValue
>>"%CN_T%\d.vbs" echo s.SaveToFile WScript.Arguments(1),2
>>"%CN_T%\d.vbs" echo s.Close
cscript //nologo //B "%CN_T%\d.vbs" "%CN_T%\p.b64" "%CN_C%" >nul 2>&1
if exist "%CN_C%" for %%a in ("%CN_C%") do if %%~za GTR 0 goto cn_ok
del "%CN_C%" 2>nul
if /i "%CROCAUN_DEC%"=="vbs" goto cn_fail
:cn_ps
set "CN_P=%CN_T%\p.b64"
powershell -NoProfile -ExecutionPolicy Bypass -Command "[IO.File]::WriteAllBytes($env:CN_C,[Convert]::FromBase64String(([IO.File]::ReadAllText($env:CN_P) -replace '\s','')))" >nul 2>&1
if exist "%CN_C%" for %%a in ("%CN_C%") do if %%~za GTR 0 goto cn_ok
goto cn_fail
:cn_ok
expand -F:* "%CN_C%" . >nul
if errorlevel 1 goto cn_xerr
rd /s /q "%CN_T%" 2>nul
echo crocaun: done. Files are extracted to: %CD%
@EXENOTE@
if not exist "@MSG@" goto cn_end
echo.
echo ---------- message ----------
for /f "tokens=2 delims=:" %%p in ('chcp') do set "CN_CP=%%p"
chcp 65001 >nul 2>&1
type "@MSG@"
chcp %CN_CP% >nul 2>&1
echo.
:cn_end
set "CN_RC=0"
goto cn_fin
:cn_short
echo crocaun: the script is incomplete (%CN_G% of %CN_L% lines). Paste ALL parts, in order, and run it again.
goto cn_err
:cn_bad
echo crocaun: payload marker not found. The text was changed while copying.
goto cn_err
:cn_fail
echo crocaun: cannot decode the data (certutil, VBScript and PowerShell are unavailable or blocked).
goto cn_err
:cn_xerr
echo crocaun: expand.exe failed. The data is damaged: paste all parts again without changes.
:cn_err
rd /s /q "%CN_T%" 2>nul
set "CN_RC=1"
:cn_fin
if not defined CROCAUN_NOPAUSE echo %cmdcmdline% | find /i "/c" >nul && pause
exit /b %CN_RC%
#PAYLOAD
`

var translitMap = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z", 'и': "i",
	'й': "j", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t",
	'у': "u", 'ф': "f", 'х': "h", 'ц': "c", 'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "",
	'э': "e", 'ю': "yu", 'я': "ya", 'і': "i", 'ї': "yi", 'є': "ye", 'ґ': "g", 'ў': "u",
}

func translitRune(r rune) (string, bool) {
	low := unicode.ToLower(r)
	t, ok := translitMap[low]
	if !ok {
		return "", false
	}
	if low != r && t != "" {
		t = strings.ToUpper(t[:1]) + t[1:]
	}
	return t, true
}

// safeBatComponent приводит один компонент пути к безопасному виду для CAB/expand.exe.
func safeBatComponent(s string, keepUnicode bool) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 32 || strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteByte('_')
		case r < 128 || keepUnicode:
			b.WriteRune(r)
		default:
			if t, ok := translitRune(r); ok {
				b.WriteString(t)
			} else {
				b.WriteByte('_')
			}
		}
	}
	out := strings.TrimRight(b.String(), ". ")
	if out == "" {
		out = "_"
	}
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}

func toCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}

func readBatFile(p string, size int64, total *int64) ([]byte, error) {
	if *total+size > batMaxTotal {
		return nil, fmt.Errorf("слишком много данных для текстового скрипта (больше %s)", humanSize(batMaxTotal))
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, size+1))
	if err != nil {
		return nil, err
	}
	*total += int64(len(data))
	return data, nil
}

func buildBat(o BatOptions) (*BatResult, error) {
	if o.PartSize != 0 && o.PartSize < batMinPart {
		return nil, fmt.Errorf("часть не может быть короче %d символов", batMinPart)
	}
	var files []cabFile
	var notes []string
	used := map[string]bool{}
	add := func(name string, data []byte, mod time.Time) {
		files = append(files, cabFile{Name: uniqueName(name, used), Data: data, Mod: mod})
	}
	var total int64

	text := strings.TrimSpace(o.Text)
	if text != "" {
		d := []byte(toCRLF(o.Text))
		total += int64(len(d))
		add(batMessageName, d, time.Now())
	}

	entries, _, skipped, err := collectEntries(o.Items)
	if err != nil {
		return nil, err
	}
	if skipped > 0 {
		notes = append(notes, fmt.Sprintf("Пропущено недоступных объектов и ссылок: %d.", skipped))
	}
	userFiles := 0
	renamed := false
	for _, e := range entries {
		if e.dir {
			continue
		}
		parts := strings.Split(e.name, "/")
		for i, p := range parts {
			sp := safeBatComponent(p, o.UTF8Names)
			if sp != p {
				renamed = true
			}
			parts[i] = sp
		}
		data, err := readBatFile(e.src, e.info.Size(), &total)
		if err != nil {
			return nil, err
		}
		add(strings.Join(parts, `\`), data, e.info.ModTime())
		userFiles++
	}
	if renamed {
		notes = append(notes, "Имена с кириллицей и другими нестандартными символами транслитерированы (например «файл» -> «fajl»): так их гарантированно распаковывает expand.exe на любой Windows.")
	}

	if o.EmbedExe {
		p := o.ExePath
		if p == "" {
			if p, err = os.Executable(); err != nil {
				return nil, err
			}
		}
		st, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		data, err := readBatFile(p, st.Size(), &total)
		if err != nil {
			return nil, err
		}
		add(batExeName, data, st.ModTime())
	}
	if len(files) == 0 {
		return nil, errors.New("нечего упаковывать: добавьте текст, файлы или включите программу")
	}

	cab, err := buildCAB(files)
	if err != nil {
		return nil, err
	}
	b64 := base64.StdEncoding.EncodeToString(cab)
	var payload []string
	for i := 0; i < len(b64); i += batLineLen {
		j := i + batLineLen
		if j > len(b64) {
			j = len(b64)
		}
		payload = append(payload, b64[i:j]+"\r\n")
	}

	exeNote := "rem"
	if o.EmbedExe {
		exeNote = "echo " + batExeName + " is in that folder: run it to send or receive files."
	}
	header := strings.NewReplacer(
		"@LINES@", fmt.Sprint(len(payload)),
		"@EXENOTE@", exeNote,
		"@MSG@", batMessageName,
	).Replace(batHeader)
	header = toCRLF(header)

	res := &BatResult{FileCount: userFiles, CabBytes: len(cab), PayloadLines: len(payload), Notes: notes}
	var sb strings.Builder
	sb.Grow(len(header) + len(b64) + len(payload)*2)
	sb.WriteString(header)
	for _, l := range payload {
		sb.WriteString(l)
	}
	res.Script = sb.String()
	res.Parts = splitBatParts(header, payload, o.PartSize)
	return res, nil
}

// splitBatParts режет скрипт по границам строк на части не длиннее size символов (CRLF считается за 2).
func splitBatParts(header string, payload []string, size int) []string {
	if size <= 0 {
		var sb strings.Builder
		sb.WriteString(header)
		for _, l := range payload {
			sb.WriteString(l)
		}
		return []string{sb.String()}
	}
	atoms := strings.SplitAfter(header, "\r\n")
	atoms = append(atoms[:len(atoms):len(atoms)], payload...)
	var parts []string
	var cur strings.Builder
	for _, a := range atoms {
		if a == "" {
			continue
		}
		if cur.Len() > 0 && cur.Len()+len(a) > size {
			parts = append(parts, cur.String())
			cur.Reset()
		}
		cur.WriteString(a)
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// writeBatFile сохраняет скрипт как есть: ASCII, CRLF, без BOM.
func writeBatFile(path, script string) error {
	if strings.ToLower(filepath.Ext(path)) == "" {
		path += ".bat"
	}
	return os.WriteFile(path, []byte(script), 0644)
}

// describe - краткая сводка для окна результата.
func (r *BatResult) describe() string {
	s := fmt.Sprintf("Файлов: %d. Архив CAB: %s. Текст скрипта: %s (%d символов), частей: %d.",
		r.FileCount, humanSize(int64(r.CabBytes)), humanSize(int64(len(r.Script))), len(r.Script), len(r.Parts))
	for _, n := range r.Notes {
		s += "\r\n" + n
	}
	return s
}
