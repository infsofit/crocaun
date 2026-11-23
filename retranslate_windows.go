//go:build windows

package main

import "github.com/lxn/walk"

// Смена языка «на лету»: обходим готовые окна и заменяем тексты (надписи, кнопки, подсказки, заголовки вкладок,
// списки) по словарю. Строки, собранные программой динамически (состояния, результаты), обновятся при следующем
// событии.

var reverseDict map[string]string

// transMap - словарь замен для перехода на язык to.
func transMap(to string) map[string]string {
	if to == "en" {
		return enDict
	}
	if reverseDict == nil {
		reverseDict = make(map[string]string, len(enDict))
		for ru, en := range enDict {
			if _, dup := reverseDict[en]; !dup {
				reverseDict[en] = ru
			}
		}
	}
	return reverseDict
}

type tipper interface {
	ToolTipText() string
	SetToolTipText(string) error
}

func retranslateWindow(mw *walk.MainWindow, to string) {
	m := transMap(to)
	tr := func(s string) string {
		if v, ok := m[s]; ok {
			return v
		}
		return s
	}
	var visit func(w walk.Widget)
	visitContainer := func(c walk.Container) {
		if c == nil || c.Children() == nil {
			return
		}
		l := c.Children()
		for i := 0; i < l.Len(); i++ {
			visit(l.At(i))
		}
	}
	visit = func(w walk.Widget) {
		if w == nil {
			return
		}
		if t, ok := w.(tipper); ok {
			if s := t.ToolTipText(); s != "" {
				if n := tr(s); n != s {
					_ = t.SetToolTipText(n)
				}
			}
		}
		switch v := w.(type) {
		case *walk.Label:
			if n := tr(v.Text()); n != v.Text() {
				_ = v.SetText(n)
			}
		case *walk.PushButton:
			if n := tr(v.Text()); n != v.Text() {
				_ = v.SetText(n)
			}
		case *walk.CheckBox:
			if n := tr(v.Text()); n != v.Text() {
				_ = v.SetText(n)
			}
		case *walk.RadioButton:
			if n := tr(v.Text()); n != v.Text() {
				_ = v.SetText(n)
			}
		case *walk.GroupBox:
			if n := tr(v.Title()); n != v.Title() {
				_ = v.SetTitle(n)
			}
		case *walk.ComboBox:
			if items, ok := v.Model().([]string); ok {
				changed := false
				out := make([]string, len(items))
				for i, s := range items {
					out[i] = tr(s)
					changed = changed || out[i] != s
				}
				if changed {
					idx, txt := v.CurrentIndex(), v.Text()
					_ = v.SetModel(out)
					if idx >= 0 && idx < len(out) {
						_ = v.SetCurrentIndex(idx)
					} else {
						_ = v.SetText(tr(txt))
					}
				}
			}
		case *walk.TabWidget:
			pages := v.Pages()
			for i := 0; i < pages.Len(); i++ {
				pg := pages.At(i)
				if n := tr(pg.Title()); n != pg.Title() {
					_ = pg.SetTitle(n)
				}
				visitContainer(pg)
			}
			return
		}
		if c, ok := w.(walk.Container); ok {
			visitContainer(c)
		}
	}
	if n := tr(mw.Title()); n != mw.Title() {
		_ = mw.SetTitle(n)
	}
	visitContainer(mw)
}
