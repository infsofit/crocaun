//go:build windows

package main

import (
	"fmt"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
)

// Варианты размера части в выпадающем списке (символов; 0 - не делить). 4000 - с запасом под лимит 4096 в Telegram.
var batPartSizes = []int{0, 4000, 8000, 16000}

// showBatDialog показывает готовый скрипт: по частям (чтобы вставлять в мессенджер) и целиком.
func showBatDialog(owner walk.Form, res *BatResult) {
	var dlg *walk.Dialog
	var partCB *walk.ComboBox
	var te *walk.TextEdit
	var statusLbl *walk.Label
	var closePB *walk.PushButton
	cur := 0

	labels := make([]string, len(res.Parts))
	for i, p := range res.Parts {
		labels[i] = fmt.Sprintf(T("Часть %d из %d (%d символов)"), i+1, len(res.Parts), len(p))
	}
	// В поле показываем не больше этого количества символов: многомегабайтный текст окно не потянет.
	const showLimit = 60000
	partText := func(i int) string {
		p := res.Parts[i]
		if len(p) > showLimit {
			p = p[:showLimit] + T("\r\n... (показано только начало: используйте кнопки копирования и сохранения)")
		}
		return p
	}
	say := func(s string) { _ = statusLbl.SetText(s) }

	_, _ = Dialog{
		AssignTo: &dlg, Title: T("Самораспаковывающийся BAT-скрипт"), DefaultButton: &closePB, CancelButton: &closePB,
		MinSize: Size{Width: 620, Height: 460}, Layout: VBox{},
		Children: []Widget{
			Label{Text: res.describe() + "\r\n" + batHowTo},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: T("Показать:")},
				ComboBox{AssignTo: &partCB, Model: labels, CurrentIndex: 0, MinSize: Size{Width: 220},
					OnCurrentIndexChanged: func() {
						// при создании окна обработчик может сработать раньше, чем появится поле текста
						if te == nil || partCB == nil {
							return
						}
						if i := partCB.CurrentIndex(); i >= 0 && i < len(res.Parts) {
							cur = i
							_ = te.SetText(partText(i))
						}
					}},
				HSpacer{},
			}},
			TextEdit{AssignTo: &te, Text: partText(0), ReadOnly: true, VScroll: true, HScroll: true, Font: Font{Family: "Consolas", PointSize: 8}, MinSize: Size{Width: 560, Height: 220}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: T("Копировать эту часть"), OnClicked: func() {
					if err := walk.Clipboard().SetText(res.Parts[cur]); err != nil {
						say(T("Не удалось скопировать: ") + err.Error())
						return
					}
					say(fmt.Sprintf(T("Часть %d из %d скопирована в буфер обмена."), cur+1, len(res.Parts)))
					// удобно для отправки по порядку: сразу выбираем следующую часть
					if cur+1 < len(res.Parts) {
						_ = partCB.SetCurrentIndex(cur + 1)
					}
				}},
				PushButton{Text: T("Копировать всё"), OnClicked: func() {
					if err := walk.Clipboard().SetText(res.Script); err != nil {
						say(T("Не удалось скопировать: ") + err.Error())
						return
					}
					say(T("Скрипт целиком скопирован в буфер обмена."))
				}},
				PushButton{Text: T("Сохранить как .bat..."), OnClicked: func() {
					fd := new(walk.FileDialog)
					fd.Title = T("Сохранить скрипт")
					fd.Filter = T("BAT-скрипт (*.bat)|*.bat|Все файлы (*.*)|*.*")
					fd.FilePath = "crocaun-script.bat"
					if ok, err := fd.ShowSave(dlg); err != nil || !ok {
						return
					}
					if err := writeBatFile(fd.FilePath, res.Script); err != nil {
						say(T("Не удалось сохранить: ") + err.Error())
						return
					}
					say(T("Сохранено: ") + fd.FilePath)
				}},
				HSpacer{},
				PushButton{AssignTo: &closePB, Text: T("Закрыть"), OnClicked: func() { dlg.Accept() }},
			}},
			Label{AssignTo: &statusLbl, Text: ""},
		},
	}.Run(owner)
}
