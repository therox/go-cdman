// Package ega — программная модель видеоадаптера EGA в том объёме, в каком
// им пользовалась CD-MAN: графический режим 640x350x16 (режим 10h) с четырьмя
// битовыми плоскостями по 64 КБ и защёлками, графический контроллер (порт 3CEh),
// палитра контроллера атрибутов и текстовый режим 80x25 с двумя наборами
// символов 8x14.
//
// Игра рисует, копируя байты видеопамяти между страницами через защёлки
// (режим записи 1), поэтому модель повторяет поведение железа побайтно.
// Видимая страница всегда начинается со смещения 0: игра не меняла адрес
// начала экрана.
package ega

import (
	"image"
	"image/color"
	"sync"
)

// Размеры экрана и видеопамяти.
const (
	W         = 640
	H         = 350
	PlaneSize = 0x10000
	PageSize  = 80 * H // 0x6D60 байт на страницу в каждой плоскости
	TextSize  = 0x8000 // сегмент B800h
	FontRows  = 14     // высота символа в режиме 350 строк
	Cols      = 80
	Rows      = 25
)

// Регистры графического контроллера.
const (
	GCSetReset    = 0
	GCEnableSR    = 1
	GCColorCmp    = 2
	GCFunction    = 3
	GCReadMap     = 4
	GCMode        = 5
	GCMisc        = 6
	GCColorDCare  = 7
	GCBitMask     = 8
	gcRegisters   = 9
	defaultCursor = 0x0B0C
)

// DefaultPalette — палитра EGA после установки режима BIOS.
var DefaultPalette = [16]byte{0, 1, 2, 3, 4, 5, 0x14, 7, 0x38, 0x39, 0x3A, 0x3B, 0x3C, 0x3D, 0x3E, 0x3F}

// Card — видеоадаптер.
type Card struct {
	mu sync.Mutex

	planes [4][PlaneSize]byte
	latch  [4]byte
	gc     [gcRegisters]byte
	pal    [16]byte

	graphics bool
	blink    bool

	text     [TextSize]byte
	fonts    [8][]byte // nil — шрифт ПЗУ
	romFont  []byte
	fontSel  byte
	cursor   uint16 // строка<<8 | столбец
	cursorSh uint16 // начальная<<8 | конечная строка курсора (бит 13 — скрыт)
}

// New создаёт адаптер со шрифтом ПЗУ rom (256 символов по 14 байт)
// в текстовом режиме 3.
func New(rom []byte) *Card {
	c := &Card{romFont: rom}
	c.setMode(3)
	return c
}

// SetMode — INT 10h, AH=0. Бит 7 в mode отменяет очистку видеопамяти.
func (c *Card) SetMode(mode byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setMode(mode)
}

func (c *Card) setMode(mode byte) {
	keep := mode&0x80 != 0
	mode &= 0x7F
	c.graphics = mode >= 0x0D
	c.gc = [gcRegisters]byte{}
	c.gc[GCColorDCare] = 0x0F
	c.gc[GCBitMask] = 0xFF
	c.pal = DefaultPalette
	c.blink = true
	c.fontSel = 0
	c.cursor = 0
	c.cursorSh = defaultCursor
	if keep {
		return
	}
	if c.graphics {
		for p := range c.planes {
			clear(c.planes[p][:])
		}
		return
	}
	for i := 0; i < Cols*Rows*2; i += 2 {
		c.text[i] = ' '
		c.text[i+1] = 7
	}
}

// Graphics сообщает, установлен ли графический режим.
func (c *Card) Graphics() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.graphics
}

// OutGC — запись в порт 3CEh словом: младший байт — номер регистра, старший — значение.
func (c *Card) OutGC(ax uint16) {
	c.SetGC(byte(ax), byte(ax>>8))
}

// SetGC задаёт регистр графического контроллера.
func (c *Card) SetGC(index, value byte) {
	c.mu.Lock()
	if int(index) < gcRegisters {
		c.gc[index] = value
	}
	c.mu.Unlock()
}

// Read читает байт видеопамяти (A000:off) и загружает защёлки (режим чтения 0).
func (c *Card) Read(off uint16) byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.read(off)
}

func (c *Card) read(off uint16) byte {
	for p := 0; p < 4; p++ {
		c.latch[p] = c.planes[p][off]
	}
	return c.latch[c.gc[GCReadMap]&3]
}

// Write записывает байт видеопамяти (A000:off) с учётом режима записи,
// set/reset, логической функции и битовой маски.
func (c *Card) Write(off uint16, v byte) {
	c.mu.Lock()
	c.write(off, v)
	c.mu.Unlock()
}

func (c *Card) write(off uint16, v byte) {
	mode := c.gc[GCMode] & 3
	fn := (c.gc[GCFunction] >> 3) & 3
	rot := c.gc[GCFunction] & 7
	mask := c.gc[GCBitMask]
	sr, esr := c.gc[GCSetReset], c.gc[GCEnableSR]
	if rot != 0 {
		v = v>>rot | v<<(8-rot)
	}
	for p := 0; p < 4; p++ {
		var d byte
		switch mode {
		case 1:
			c.planes[p][off] = c.latch[p]
			continue
		case 2:
			if v&(1<<p) != 0 {
				d = 0xFF
			}
		case 3:
			if sr&(1<<p) != 0 {
				d = 0xFF
			}
		default:
			d = v
			if esr&(1<<p) != 0 {
				d = 0
				if sr&(1<<p) != 0 {
					d = 0xFF
				}
			}
		}
		l := c.latch[p]
		switch fn {
		case 1:
			d &= l
		case 2:
			d |= l
		case 3:
			d ^= l
		}
		m := mask
		if mode == 3 {
			m &= v
		}
		c.planes[p][off] = d&m | l&^m
	}
}

// Copy — «mov al, es:[src]; mov es:[dst], al»: в режиме записи 1 копирует
// байт всех четырёх плоскостей через защёлки.
func (c *Card) Copy(src, dst uint16) {
	c.mu.Lock()
	c.read(src)
	c.write(dst, 0)
	c.mu.Unlock()
}

// CopyBlock копирует прямоугольник w байт на h строк со смещения src на dst
// построчно с шагом 80 байт (так устроены почти все процедуры рисования игры).
func (c *Card) CopyBlock(src, dst uint16, w, h int) {
	c.mu.Lock()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c.read(src + uint16(x))
			c.write(dst+uint16(x), 0)
		}
		src += 80
		dst += 80
	}
	c.mu.Unlock()
}

// Plane возвращает байт плоскости p без загрузки защёлок (для отладки и тестов).
func (c *Card) Plane(p int, off uint16) byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.planes[p&3][off]
}

// SetPalette задаёт 16 регистров палитры контроллера атрибутов.
func (c *Card) SetPalette(p [16]byte) {
	c.mu.Lock()
	for i := range p {
		c.pal[i] = p[i] & 0x3F
	}
	c.mu.Unlock()
}

// SetPaletteReg — INT 10h, AX=1000h: регистр i получает цвет v.
func (c *Card) SetPaletteReg(i, v byte) {
	c.mu.Lock()
	if i < 16 {
		c.pal[i] = v & 0x3F
	}
	c.mu.Unlock()
}

// Palette возвращает текущую палитру.
func (c *Card) Palette() [16]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pal
}

// SetBlink — INT 10h, AX=1003h: мигание (true) или 16 цветов фона (false).
func (c *Card) SetBlink(on bool) {
	c.mu.Lock()
	c.blink = on
	c.mu.Unlock()
}

// LoadFont — INT 10h, AX=1110h: count символов высотой h начиная с first в блок block.
func (c *Card) LoadFont(block int, data []byte, first, count, h int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	block &= 7
	if c.fonts[block] == nil {
		c.fonts[block] = append([]byte(nil), c.romFont...)
	}
	f := c.fonts[block]
	for i := 0; i < count && first+i < 256; i++ {
		for r := 0; r < FontRows; r++ {
			var v byte
			if r < h && i*h+r < len(data) {
				v = data[i*h+r]
			}
			f[(first+i)*FontRows+r] = v
		}
	}
}

// SelectFonts — INT 10h, AX=1103h: какие блоки шрифта используются для символов
// с атрибутом без бита яркости (A) и с ним (B).
func (c *Card) SelectFonts(bl byte) {
	c.mu.Lock()
	c.fontSel = bl
	c.mu.Unlock()
}

// TextW записывает слово (символ и атрибут) в текстовую память B800:off.
func (c *Card) TextW(off uint16, w uint16) {
	c.mu.Lock()
	c.text[off&(TextSize-1)] = byte(w)
	c.text[(off+1)&(TextSize-1)] = byte(w >> 8)
	c.mu.Unlock()
}

// TextRW читает слово текстовой памяти.
func (c *Card) TextRW(off uint16) uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return uint16(c.text[off&(TextSize-1)]) | uint16(c.text[(off+1)&(TextSize-1)])<<8
}

// TextB записывает байт текстовой памяти.
func (c *Card) TextB(off uint16, b byte) {
	c.mu.Lock()
	c.text[off&(TextSize-1)] = b
	c.mu.Unlock()
}

// TextRB читает байт текстовой памяти.
func (c *Card) TextRB(off uint16) byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.text[off&(TextSize-1)]
}

// SetCursor — INT 10h, AH=2 (DH строка, DL столбец).
func (c *Card) SetCursor(dx uint16) {
	c.mu.Lock()
	c.cursor = dx
	c.mu.Unlock()
}

// SetCursorShape — INT 10h, AH=1.
func (c *Card) SetCursorShape(cx uint16) {
	c.mu.Lock()
	c.cursorSh = cx
	c.mu.Unlock()
}

// WriteChar — INT 10h, AH=9: символ ch с атрибутом attr count раз с позиции курсора.
func (c *Card) WriteChar(ch, attr byte, count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pos := (int(c.cursor>>8)*Cols + int(c.cursor&0xFF)) * 2
	for i := 0; i < count && pos < Cols*Rows*2; i++ {
		c.text[pos] = ch
		c.text[pos+1] = attr
		pos += 2
	}
}

// Teletype — INT 10h, AH=0Eh: вывести символ с позиции курсора и сдвинуть его.
// В графическом режиме символ рисуется шрифтом ПЗУ цветом color.
func (c *Card) Teletype(ch, color byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	row, col := int(c.cursor>>8), int(c.cursor&0xFF)
	switch ch {
	case '\r':
		col = 0
	case '\n':
		row++
	case 8:
		if col > 0 {
			col--
		}
	default:
		if c.graphics {
			for r := 0; r < FontRows; r++ {
				off := (row*FontRows+r)*80 + col
				if off >= PlaneSize {
					break
				}
				bits := c.romFont[int(ch)*FontRows+r]
				for p := 0; p < 4; p++ {
					if color&(1<<p) != 0 {
						c.planes[p][off] = bits
					} else {
						c.planes[p][off] = 0
					}
				}
			}
		} else {
			i := (row*Cols + col) * 2
			c.text[i] = ch
		}
		col++
		if col >= Cols {
			col = 0
			row++
		}
	}
	if row >= Rows {
		row = Rows - 1
	}
	c.cursor = uint16(row)<<8 | uint16(col)
}

// Frame — снимок экрана: индексы цветов 0..15 и палитра.
type Frame struct {
	Pix [W * H]byte
	Pal [16]byte
}

// Capture снимает видимое изображение в f. blinkPhase — фаза мигания курсора.
func (c *Card) Capture(f *Frame, blinkPhase bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	f.Pal = c.pal
	if c.graphics {
		for y := 0; y < H; y++ {
			row := f.Pix[y*W:]
			for xb := 0; xb < 80; xb++ {
				off := y*80 + xb
				b0, b1, b2, b3 := c.planes[0][off], c.planes[1][off], c.planes[2][off], c.planes[3][off]
				for bit := 0; bit < 8; bit++ {
					s := 7 - bit
					row[xb*8+bit] = (b0>>s)&1 | ((b1>>s)&1)<<1 | ((b2>>s)&1)<<2 | ((b3>>s)&1)<<3
				}
			}
		}
		return
	}
	sel := c.fontSel
	blkA := sel&3 | (sel>>2)&4
	blkB := (sel>>2)&3 | (sel>>3)&4
	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			i := (row*Cols + col) * 2
			ch, at := c.text[i], c.text[i+1]
			blk := blkA
			if at&8 != 0 {
				blk = blkB
			}
			font := c.romFont
			if blk != 0 && c.fonts[blk] != nil {
				font = c.fonts[blk]
			}
			fg, bg := at&15, at>>4
			if c.blink {
				bg &= 7
				if at&0x80 != 0 && !blinkPhase {
					fg = bg
				}
			}
			for r := 0; r < FontRows; r++ {
				bits := font[int(ch)*FontRows+r]
				o := (row*FontRows+r)*W + col*8
				for x := 0; x < 8; x++ {
					if bits&(0x80>>x) != 0 {
						f.Pix[o+x] = fg
					} else {
						f.Pix[o+x] = bg
					}
				}
			}
		}
	}
	// курсор: подчёркивание в двух нижних строках символа
	if c.cursorSh&0x2000 == 0 && blinkPhase {
		row, col := int(c.cursor>>8), int(c.cursor&0xFF)
		if row < Rows && col < Cols {
			fg := c.text[(row*Cols+col)*2+1] & 15
			for r := FontRows - 3; r < FontRows-1; r++ {
				o := (row*FontRows+r)*W + col*8
				for x := 0; x < 8; x++ {
					f.Pix[o+x] = fg
				}
			}
		}
	}
}

// Color переводит 6-битный цвет EGA (rgbRGB) в RGB.
func Color(c byte) color.RGBA {
	v := func(hi, lo byte) uint8 { return 0xAA*hi + 0x55*lo }
	return color.RGBA{
		R: v((c>>2)&1, (c>>5)&1),
		G: v((c>>1)&1, (c>>4)&1),
		B: v(c&1, (c>>3)&1),
		A: 0xFF,
	}
}

// RGBA переводит кадр в RGBA (буфер W*H*4).
func (f *Frame) RGBA(dst []byte) {
	var lut [16][4]byte
	for i, c := range f.Pal {
		rgb := Color(c)
		lut[i] = [4]byte{rgb.R, rgb.G, rgb.B, 0xFF}
	}
	for i, ci := range f.Pix {
		copy(dst[i*4:i*4+4], lut[ci&15][:])
	}
}

// Image возвращает кадр как изображение.
func (f *Frame) Image() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, W, H))
	f.RGBA(img.Pix)
	return img
}

// Snapshot снимает текущий экран как изображение (для отладки и тестов).
func (c *Card) Snapshot() *image.RGBA {
	var f Frame
	c.Capture(&f, false)
	return f.Image()
}
