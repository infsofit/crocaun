package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Порты с автоподбором. У каждого порта есть «слоты»: слот 0 - базовый порт (его занимает первая копия программы,
// так что со старыми версиями и Android всё работает как раньше), слот N - базовый порт + N*100. Следующие копии
// программы (другие профили -w) автоматически берут первый свободный слот. Поиск устройств опрашивает все слоты,
// а в ответе на запрос передаётся реальный TCP-порт (это было в протоколе и раньше).
const (
	portSlots        = 10  // сколько слотов пробуем
	portSlotStep     = 100 // шаг между слотами
	scanUnicastSlots = 4   // при переборе адресов подсети (по одному адресу) опрашиваем первые слоты; широковещательно - все
)

func slotPort(base, slot int) int { return base + slot*portSlotStep }

// slotPorts - порты первых n слотов базового порта.
func slotPorts(base, n int) []int {
	out := make([]int, 0, n)
	for i := 0; i < n && slotPort(base, i) < 65536; i++ {
		out = append(out, slotPort(base, i))
	}
	return out
}

// isSlotPort: port - один из слотов базового порта base (используется в тестах).
func isSlotPort(base, port int) bool {
	d := port - base
	return d >= 0 && d%portSlotStep == 0 && d/portSlotStep < portSlots
}

// listenUDPSlot открывает UDP-порт в первом свободном слоте.
func listenUDPSlot(base int) (*net.UDPConn, int, error) {
	var last error
	for _, p := range slotPorts(base, portSlots) {
		c, err := net.ListenUDP("udp4", &net.UDPAddr{Port: p})
		if err == nil {
			return c, p, nil
		}
		last = err
	}
	return nil, 0, last
}

// listenTCPSlot открывает TCP-порт в первом свободном слоте.
func listenTCPSlot(base int) (net.Listener, int, error) {
	var last error
	for _, p := range slotPorts(base, portSlots) {
		l, err := net.Listen("tcp4", fmt.Sprintf(":%d", p))
		if err == nil {
			return l, p, nil
		}
		last = err
	}
	return nil, 0, last
}

// pickRelayBlock выбирает блок портов relay (lanRelayPorts, сдвинутый на слот), в котором свободны все порты.
// Базовый блок получает несколько попыток: после остановки режима обмена порты освобождаются не мгновенно, а
// адрес устройства лучше сохранять прежним.
func pickRelayBlock() ([]string, error) {
	var last error
	try := func(slot int) []string {
		var ports []string
		var ls []net.Listener
		ok := true
		for _, bp := range lanRelayPorts {
			p := mustAtoi(bp) + slot*portSlotStep
			l, err := net.Listen("tcp4", fmt.Sprintf(":%d", p))
			if err != nil {
				last, ok = err, false
				break
			}
			ls = append(ls, l)
			ports = append(ports, strconv.Itoa(p))
		}
		for _, l := range ls {
			_ = l.Close()
		}
		if ok {
			return ports
		}
		return nil
	}
	for i := 0; i < 20; i++ { // до ~3 секунд для базового блока
		if p := try(0); p != nil {
			return p, nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	for slot := 1; slot < portSlots; slot++ {
		if p := try(slot); p != nil {
			return p, nil
		}
	}
	return nil, last
}

// portRanges - список диапазонов для брандмауэра: базовые порты base..base+count-1 во всех слотах ("29009-29013,29109-29113,...").
func portRanges(base, count int) string {
	var parts []string
	for _, p := range slotPorts(base, portSlots) {
		if count <= 1 {
			parts = append(parts, strconv.Itoa(p))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", p, p+count-1))
		}
	}
	return strings.Join(parts, ",")
}
