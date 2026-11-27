package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Параметр запуска -w / --workdir <папка>: все рабочие файлы (crocaun.ini, папка чата, Share, приём) лежат в этой папке,
// а не рядом с exe. Так из одного exe запускается несколько независимых «профилей».

var envPercent = regexp.MustCompile(`%([^%\s]+)%`)

// expandEnvPercent подставляет %VAR% (cmd.exe делает это сам, но ярлыки и другие запускающие программы - нет).
func expandEnvPercent(s string) string {
	return envPercent.ReplaceAllStringFunc(s, func(m string) string {
		if v, ok := os.LookupEnv(m[1 : len(m)-1]); ok {
			return v
		}
		return m
	})
}

// parseWorkdirArg вырезает из аргументов -w/--workdir (формы "-w X", "--workdir X", "-w=X", "--workdir=X").
// Возвращает оставшиеся аргументы, папку и ошибку (параметр без значения).
func parseWorkdirArg(args []string) (rest []string, dir string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-w" || a == "--workdir" || a == "/w":
			if i+1 >= len(args) || strings.TrimSpace(args[i+1]) == "" {
				return nil, "", fmt.Errorf(T("после %s нужно указать папку"), a)
			}
			dir = args[i+1]
			i++
		case strings.HasPrefix(a, "--workdir="):
			dir = strings.TrimPrefix(a, "--workdir=")
		case strings.HasPrefix(a, "-w="):
			dir = strings.TrimPrefix(a, "-w=")
		default:
			rest = append(rest, a)
			continue
		}
		if strings.TrimSpace(dir) == "" {
			return nil, "", fmt.Errorf(T("пустая папка в параметре %s"), a)
		}
	}
	return rest, dir, nil
}

// applyWorkdir обрабатывает параметр до разбора режимов. Для внутреннего режима --croc параметр не трогаем.
func applyWorkdir() error {
	if len(os.Args) > 1 && os.Args[1] == "--croc" {
		return nil
	}
	rest, dir, err := parseWorkdirArg(os.Args[1:])
	if err != nil {
		return err
	}
	if dir == "" {
		return nil
	}
	dir = expandEnvPercent(strings.Trim(dir, `"`))
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return fmt.Errorf(T("не удалось создать рабочую папку %s: %v"), abs, err)
	}
	exeDirOverride = abs
	os.Args = append([]string{os.Args[0]}, rest...)
	return nil
}
