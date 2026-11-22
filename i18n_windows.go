//go:build windows

package main

import "syscall"

// sysUILangRussian: основной язык интерфейса Windows - русский (LANGID & 0x3ff == 0x19).
func sysUILangRussian() bool {
	p := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")
	r, _, _ := p.Call()
	return r&0x3ff == 0x19
}
