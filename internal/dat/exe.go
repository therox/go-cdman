// Package dat читает файлы оригинальной игры CD-MAN 2.0: сегмент данных
// программы CDMAN2.EXE и картинки .CDM.
package dat

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

// Устройство CDMAN2.EXE (35 337 байт, 08.06.1992): заголовок MZ 512 байт,
// за ним образ: стек (сегмент 0, 0x200 байт), сегмент данных (параграф 0x20)
// и сегмент кода (параграф 0x376).
const (
	exeSize   = 35337
	hdrSize   = 0x200
	dsPara    = 0x20
	csPara    = 0x376
	DSSize    = (csPara - dsPara) * 16 // 0x3560
	exeIP     = 0
	exeRelocs = 1
)

// ErrFormat — файл не похож на CDMAN2.EXE версии 2.0.
var ErrFormat = errors.New("файл не похож на CDMAN2.EXE игры CD-MAN 2.0")

// signatures — строки сегмента данных, по которым узнаётся нужная версия.
var signatures = []struct {
	off int
	s   string
}{
	{0x299, "TITLE   .CDM"},
	{0x390, "CHARTAB .CDM"},
	{0x3B5, "CD-MAN has been terminated."},
	{0x20D8, "   SEE      Top-10     Info      Demo    Position SPEED"},
	{0x3397, "CYGN      DRAC      ORIO"},
}

// ParseEXE извлекает начальное содержимое сегмента данных из CDMAN2.EXE.
func ParseEXE(raw []byte) ([]byte, error) {
	if len(raw) != exeSize {
		return nil, fmt.Errorf("%w: размер %d байт вместо %d", ErrFormat, len(raw), exeSize)
	}
	if raw[0] != 'M' || raw[1] != 'Z' {
		return nil, fmt.Errorf("%w: нет заголовка MZ", ErrFormat)
	}
	hdr := int(binary.LittleEndian.Uint16(raw[8:])) * 16
	cs := binary.LittleEndian.Uint16(raw[0x16:])
	ip := binary.LittleEndian.Uint16(raw[0x14:])
	nrel := binary.LittleEndian.Uint16(raw[6:])
	if hdr != hdrSize || cs != csPara || ip != exeIP || nrel != exeRelocs {
		return nil, fmt.Errorf("%w: неожиданный заголовок", ErrFormat)
	}
	img := raw[hdr:]
	ds := make([]byte, DSSize)
	copy(ds, img[dsPara*16:csPara*16])
	for _, sg := range signatures {
		if !bytes.Equal(ds[sg.off:sg.off+len(sg.s)], []byte(sg.s)) {
			return nil, fmt.Errorf("%w: не найдена строка %q", ErrFormat, sg.s)
		}
	}
	return ds, nil
}
