package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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
				return nil, "", fmt.Errorf("после %s нужно указать папку", a)
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
			return nil, "", fmt.Errorf("пустая папка в параметре %s", a)
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
		return fmt.Errorf("не удалось создать рабочую папку %s: %v", abs, err)
	}
	exeDirOverride = abs
	os.Args = append([]string{os.Args[0]}, rest...)
	return nil
}

// checkPorts проверяет порты, нужные программе, и возвращает описания занятых. Раздельные профили с одинаковыми
// портами мешают друг другу; полноценное решение - в TODO.md.
func checkPorts() []string {
	var busy []string
	// Проверка не должна вызывать запрос брандмауэра Windows, поэтому на всех адресах ничего не слушаем:
	// TCP - пробное подключение к 127.0.0.1 (отказ = порт свободен), UDP - пробная привязка к 127.0.0.1 (приблизительно).
	tcp := func(p int) bool {
		c, err := net.DialTimeout("tcp4", fmt.Sprintf("127.0.0.1:%d", p), 300*time.Millisecond)
		if err != nil {
			return true
		}
		_ = c.Close()
		return false
	}
	udp := func(p int) bool {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: p})
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}
	if !udp(lanDiscPort) {
		busy = append(busy, fmt.Sprintf("UDP %d", lanDiscPort))
	}
	free := 0
	for _, p := range lanRelayPorts {
		if n, err := fmt.Sscanf(p, "%d", new(int)); n == 1 && err == nil {
			var v int
			fmt.Sscanf(p, "%d", &v)
			if tcp(v) {
				free++
			}
		}
	}
	if free == 0 {
		busy = append(busy, "TCP "+strings.Join(lanRelayPorts, ", "))
	}
	if !tcp(chatTCPPort) {
		busy = append(busy, fmt.Sprintf("TCP %d", chatTCPPort))
	}
	if !udp(chatDiscPort) {
		busy = append(busy, fmt.Sprintf("UDP %d", chatDiscPort))
	}
	return busy
}
