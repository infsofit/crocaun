package main

import "strings"

// Строки интерфейса, которые раньше были константами: переводятся при выборе языка (initUIStrings).
var (
	destInternet, destWait, destExch, destBat string
	allDevices                                string
	batHowTo, settingsInfoText                string
	batPartLabels                             []string
)

// ruAllDevices - русское значение «Все найденные устройства» (могло быть сохранено в crocaun.ini).
const ruAllDevices = "Все найденные устройства"

func isAllDevices(t string) bool { return t == allDevices || t == ruAllDevices }

func initUIStrings() {
	destInternet = T("Через интернет")
	destWait = T("Ждать получателя в сети")
	destExch = T("Через обменники")
	destBat = T("BAT-скрипт (текст для мессенджера)")
	allDevices = T("Все найденные устройства")
	batPartLabels = []string{T("не делить"), T("до 4000 символов"), T("до 8000 символов"), T("до 16000 символов")}
	batHowTo = T("Получателю: создать пустой файл с расширением .bat (например x.bat), вставить в него ВСЕ части по порядку " +
		"без изменений, сохранить и запустить двойным щелчком. Файлы распакуются в папку со скриптом.")
	settingsInfoText = T("Настройки хранятся в crocaun.ini рядом с программой. Пароли (прокси, relay и отмеченные «запомнить» " +
		"пароли передачи) сохраняются только в зашифрованном виде. Без мастер-пароля защита привязана к вашей учётной записи " +
		"Windows на этом компьютере (на другом компьютере пароли придётся ввести заново). С мастер-паролем пароли открываются " +
		"на любом компьютере, но мастер-пароль спрашивается при запуске и его нельзя восстановить.")
	_ = strings.TrimSpace
}

// trErr - ошибка с переводимым текстом (для глобальных переменных-ошибок; сравнение через == работает).
type trErr string

func (e trErr) Error() string { return T(string(e)) }
