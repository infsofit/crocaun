// crocaun - один exe: оболочка (GUI) + встроенный croc.
// Режимы:
//
//	crocaun.exe               - окно программы
//	crocaun.exe --croc ...    - внутренний режим: работает как croc (запускается самой программой на каждую передачу)
//	crocaun.exe --selftest    - самотест передачи (для CI)
//	crocaun.exe --guitest     - тест окна (для CI)
//	crocaun.exe --battest     - тест самораспаковывающихся BAT-скриптов (для CI, Windows)
//	crocaun.exe -w папка      - все рабочие файлы (ini, чат, Share) в указанной папке (--workdir, можно %VAR%)
//	crocaun.exe --send путь.. - окно с добавленными файлами (меню Проводника «Отправить через Crocau»)
package main

import (
	"log"
	"os"

	"github.com/schollz/croc/v10/src/cli"
)

func main() {
	if err := applyWorkdir(); err != nil {
		log.Println(T("Параметр -w/--workdir:"), err)
		os.Exit(2)
	}
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--selftest", "--guitest", "--battest", "--fwtest", "--chattest", "--nettest":
			setLang("ru") // тесты сверяют русские тексты
		}
		switch os.Args[1] {
		case "--croc":
			os.Args = append([]string{"croc"}, os.Args[2:]...)
			if err := cli.Run(); err != nil {
				log.Fatalln(err)
			}
			return
		case "--selftest":
			os.Exit(selfTest())
		case "--fw-install":
			os.Exit(fwInstallMain())
		case "--fw-remove":
			os.Exit(fwRemoveMain())
		case "--battest":
			os.Exit(batTest())
		case "--fwtest":
			os.Exit(fwSelfTest())
		case "--chattest":
			os.Exit(chatSpike())
		case "--nettest":
			os.Exit(netTest())
		case "--send":
			// из меню Проводника: окно с выбранными файлами (или пересылка в уже открытое окно)
			if code := shellSendMain(os.Args[2:]); code >= 0 {
				os.Exit(code)
			}
		case "--guitest":
			os.Exit(guiMain(true))
		}
	}
	os.Exit(guiMain(false))
}
