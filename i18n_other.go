//go:build !windows

package main

import (
	"os"
	"strings"
)

func sysUILangRussian() bool {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return strings.HasPrefix(strings.ToLower(v), "ru")
		}
	}
	return false
}
