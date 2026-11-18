package main

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"time"
)

// Запись архива Microsoft Cabinet (CAB) со сжатием MSZIP (deflate блоками по 32 КБ).
// Зачем CAB, а не zip: распаковывает его expand.exe, который есть во всех Windows начиная с XP -
// не нужны ни .NET, ни PowerShell, ни Проводник (Shell.Application). Своих зависимостей нет:
// compress/flate уже входит в программу, так что размер exe почти не растёт.
//
// Формат (MS-CAB): CFHEADER(36) + CFFOLDER(8) + CFFILE[] + CFDATA[]. Все файлы лежат в одной
// «папке» (потоке), блоки сжимаются независимо друг от друга - это допустимо для MSZIP.

type cabFile struct {
	Name string // путь внутри архива, разделитель '\'
	Data []byte
	Mod  time.Time
}

const (
	cabBlockSize = 32768
	cabMaxFiles  = 65535
	cabMaxBytes  = 0x7fffffff
)

// cabChecksum - контрольная сумма CFDATA (XOR 32-битных слов, хвост добивается нулями со сдвигом).
func cabChecksum(data []byte, seed uint32) uint32 {
	cs := seed
	n := len(data) / 4
	for i := 0; i < n; i++ {
		cs ^= binary.LittleEndian.Uint32(data[i*4:])
	}
	rest := data[n*4:]
	var ul uint32
	switch len(rest) {
	case 3:
		ul = uint32(rest[0])<<16 | uint32(rest[1])<<8 | uint32(rest[2])
	case 2:
		ul = uint32(rest[0])<<8 | uint32(rest[1])
	case 1:
		ul = uint32(rest[0])
	}
	return cs ^ ul
}

func cabDOSTime(t time.Time) (uint16, uint16) {
	if t.IsZero() || t.Year() < 1980 {
		t = time.Date(1980, 1, 1, 0, 0, 0, 0, time.Local)
	}
	if t.Year() > 2107 {
		t = time.Date(2107, 12, 31, 23, 59, 58, 0, time.Local)
	}
	date := uint16((t.Year()-1980)<<9 | int(t.Month())<<5 | t.Day())
	tm := uint16(t.Hour()<<11 | t.Minute()<<5 | t.Second()/2)
	return date, tm
}

func cabHasNonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return true
		}
	}
	return false
}

// mszipBlock сжимает один блок: сигнатура "CK" + завершённый deflate-поток.
func mszipBlock(raw []byte) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("CK")
	w, err := flate.NewWriter(&b, flate.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// cabOpts - режимы записи (нужны для диагностики на разных версиях expand.exe).
type cabOpts struct {
	zeroSum bool // контрольные суммы блоков = 0 («не вычислялись», допустимо по спецификации)
	stored  bool // блоки без сжатия: deflate «stored» с флагом последнего блока
}

// mszipStored - блок MSZIP из одного несжатого deflate-блока (BFINAL=1).
func mszipStored(raw []byte) []byte {
	b := make([]byte, 0, len(raw)+7)
	b = append(b, 'C', 'K', 0x01, byte(len(raw)), byte(len(raw)>>8), ^byte(len(raw)), ^byte(len(raw)>>8))
	return append(b, raw...)
}

func buildCAB(files []cabFile) ([]byte, error) { return buildCABOpts(files, cabOpts{}) }

// buildCABOpts собирает архив из файлов. Имена с символами не из ASCII получают флаг UTF-8
// (expand.exe разных версий понимает его по-разному, поэтому вызывающий код обычно подаёт ASCII).
func buildCABOpts(files []cabFile, o cabOpts) ([]byte, error) {
	if len(files) == 0 {
		return nil, errors.New("cab: нет файлов")
	}
	if len(files) > cabMaxFiles {
		return nil, errors.New("cab: слишком много файлов (максимум 65535)")
	}

	var blocks [][]byte // готовые CFDATA (заголовок + данные)
	buf := make([]byte, 0, cabBlockSize)
	flush := func() error {
		var comp []byte
		if o.stored {
			comp = mszipStored(buf)
		} else {
			var err error
			if comp, err = mszipBlock(buf); err != nil {
				return err
			}
		}
		if len(comp) > 0xffff {
			return errors.New("cab: блок не поместился")
		}
		blk := make([]byte, 8+len(comp))
		binary.LittleEndian.PutUint16(blk[4:], uint16(len(comp)))
		binary.LittleEndian.PutUint16(blk[6:], uint16(len(buf)))
		copy(blk[8:], comp)
		sum := cabChecksum(blk[4:8], 0)
		sum = cabChecksum(comp, sum)
		if o.zeroSum {
			sum = 0
		}
		binary.LittleEndian.PutUint32(blk[0:], sum)
		blocks = append(blocks, blk)
		buf = buf[:0]
		return nil
	}

	offs := make([]uint32, len(files))
	var pos uint64
	for i, f := range files {
		if uint64(len(f.Data)) > cabMaxBytes || pos+uint64(len(f.Data)) > cabMaxBytes {
			return nil, errors.New("cab: данные больше 2 ГБ")
		}
		offs[i] = uint32(pos)
		pos += uint64(len(f.Data))
		data := f.Data
		for len(data) > 0 {
			n := cabBlockSize - len(buf)
			if n > len(data) {
				n = len(data)
			}
			buf = append(buf, data[:n]...)
			data = data[n:]
			if len(buf) == cabBlockSize {
				if err := flush(); err != nil {
					return nil, err
				}
			}
		}
	}
	if len(buf) > 0 {
		if err := flush(); err != nil {
			return nil, err
		}
	}
	if len(blocks) > 0xffff {
		return nil, errors.New("cab: данные больше 2 ГБ")
	}

	filesSize := 0
	for _, f := range files {
		filesSize += 16 + len(f.Name) + 1
	}
	blocksSize := 0
	for _, b := range blocks {
		blocksSize += len(b)
	}
	coffFiles := 36 + 8
	coffData := coffFiles + filesSize
	total := coffData + blocksSize
	if uint64(total) > cabMaxBytes {
		return nil, errors.New("cab: архив больше 2 ГБ")
	}

	out := make([]byte, 0, total)
	le := binary.LittleEndian
	u16 := func(v uint16) { out = le.AppendUint16(out, v) }
	u32 := func(v uint32) { out = le.AppendUint32(out, v) }

	// CFHEADER
	out = append(out, 'M', 'S', 'C', 'F')
	u32(0)
	u32(uint32(total))
	u32(0)
	u32(uint32(coffFiles))
	u32(0)
	out = append(out, 3, 1) // версия 1.3
	u16(1)                  // папок
	u16(uint16(len(files)))
	u16(0)      // флаги
	u16(0x4352) // setID
	u16(0)      // номер архива в наборе
	// CFFOLDER
	u32(uint32(coffData))
	u16(uint16(len(blocks)))
	u16(1) // MSZIP
	// CFFILE
	for i, f := range files {
		date, tm := cabDOSTime(f.Mod)
		attr := uint16(0x20)
		if cabHasNonASCII(f.Name) {
			attr |= 0x80
		}
		u32(uint32(len(f.Data)))
		u32(offs[i])
		u16(0)
		u16(date)
		u16(tm)
		u16(attr)
		out = append(out, f.Name...)
		out = append(out, 0)
	}
	for _, b := range blocks {
		out = append(out, b...)
	}
	return out, nil
}
