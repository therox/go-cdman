package game

import "strings"

// Имена файлов в сегменте данных (строки DOS с пробелами перед точкой).
const (
	fnTitle   = 0x0299
	fnHelp    = 0x0376 // HELP.CDM может отсутствовать
	fnHiScore = 0x0383
	fnCharTab = 0x0390
	fnDemo    = 0x039D
	fnWoshi   = 0x0369
)

// Переменные работы с файлами.
const (
	vHandle    = 0x03AA // описатель открытого файла
	vDemoFile  = 0x03AC // описатель DEMO.CDM
	vSizeLo    = 0x03AE // размер файла
	vSizeHi    = 0x03B0
	vNoHelp    = 0x03B2 // HELP.CDM не найден
	vHeader    = 0x1045 // 16 байт палитры из заголовка картинки
	vMsgFatal  = 0x03B3 // «CD-MAN has been terminated…»$
	charTabOff = 0x254D // CHARTAB.CDM (0xDE4 байт)
	hiScoreOff = 0x165B // HIGHSC.CDM (0x8AD байт)
	hiScoreLen = 0x08AD
)

// dosFile — открытый файл DOS.
type dosFile struct {
	name string
	data []byte
	pos  int
}

// dsString возвращает строку сегмента данных до нулевого байта.
func (g *Game) dsString(a uint16) string {
	var sb strings.Builder
	for g.ds[a] != 0 {
		sb.WriteByte(g.ds[a])
		a++
	}
	return sb.String()
}

// dosName переводит имя файла оригинала («TITLE   .CDM») в обычное.
func dosName(s string) string {
	return strings.ToUpper(strings.ReplaceAll(s, " ", ""))
}

// open — INT 21h, AH=3Dh: открыть файл с именем в DS:dx. Возвращает
// описатель или ok=false.
func (g *Game) open(dx uint16) (uint16, bool) {
	name := dosName(g.dsString(dx))
	data, err := g.fs.Read(name)
	if err != nil {
		return 2, false
	}
	for h := 5; h < len(g.files); h++ {
		if g.files[h] == nil {
			g.files[h] = &dosFile{name: name, data: data}
			return uint16(h), true
		}
	}
	return 4, false
}

func (g *Game) file(h uint16) *dosFile {
	if int(h) < len(g.files) {
		return g.files[h]
	}
	return nil
}

// closeFile — INT 21h, AH=3Eh.
func (g *Game) closeFile(h uint16) {
	if int(h) < len(g.files) {
		g.files[h] = nil
	}
}

// readFile — INT 21h, AH=3Fh: прочитать до n байт в dst.
func (g *Game) readFile(h uint16, dst []byte) int {
	f := g.file(h)
	if f == nil {
		return 0
	}
	n := copy(dst, f.data[min(f.pos, len(f.data)):])
	f.pos += n
	return n
}

// seekFile — INT 21h, AH=42h.
func (g *Game) seekFile(h uint16, whence int, off int) int {
	f := g.file(h)
	if f == nil {
		return 0
	}
	switch whence {
	case 0:
		f.pos = off
	case 1:
		f.pos += off
	case 2:
		f.pos = len(f.data) + off
	}
	return f.pos
}

// fatal — L_4643: файл не найден — текстовый режим, сообщение и выход.
func (g *Game) fatal() {
	g.v.SetMode(3)
	g.printDOS(vMsgFatal)
	panic(exitDOS{})
}

// printDOS — INT 21h, AH=9: вывести строку до «$» телетайпом BIOS.
func (g *Game) printDOS(a uint16) {
	for g.ds[a] != '$' {
		g.teletype(g.ds[a])
		a++
	}
}

// openFile — L_462e: открыть файл; если нет HELP.CDM — флаг g_03b2,
// иначе аварийное завершение.
func (g *Game) openFile(dx uint16) uint16 {
	h, ok := g.open(dx)
	if ok {
		return h
	}
	if dx == fnHelp {
		g.sb(vNoHelp, 1)
		return h
	}
	g.fatal()
	return 0
}

// loadFile — L_4616: прочитать файл в буфер [0294h]; у картинок первые
// 16 байт (палитра) читаются в DS:1045h.
func (g *Game) loadFile(dx uint16) {
	h := g.openFile(dx)
	if g.b(vNoHelp) == 1 {
		return
	}
	g.sw(vHandle, h)
	// L_4654
	g.seekFile(h, 0, 0)
	skip := 0
	if dx != fnHelp {
		g.readFile(h, g.ds[vHeader:vHeader+16])
		skip = 0x10
	}
	size := g.seekFile(h, 2, 0)
	g.sw(vSizeLo, uint16(size))
	g.sw(vSizeHi, uint16(size>>16))
	g.seekFile(h, 0, skip)
	base := g.bufBase()
	if g.w(vSizeHi) != 0 {
		g.readFile(h, g.buf[base:base+0xFFF0])
		base += 0xFFF0
		g.addw(vSizeLo, 0x10)
	}
	n := int(g.w(vSizeLo))
	g.readFile(h, g.buf[base:base+n])
	g.closeFile(h)
}

// decodeImage — L_46d9: распаковать картинку из буфера в видеопамять
// сегмента seg (A000h, A6D6h, ADACh). Сначала страница очищается, затем
// каждая плоскость (3, 2, 1, 0) по столбцам байт сверху вниз ORится с данными.
// Байты за пределами окна A000–AFFF (у страницы ADACh — ниже 119-й строки)
// теряются, как и на настоящем адаптере.
func (g *Game) decodeImage(seg uint16) {
	base := uint32(seg-0xA000) * 16
	vram := func(di uint16) (uint16, bool) {
		a := base + uint32(di)
		return uint16(a), a < 0x10000
	}
	v := g.v
	v.OutGC(0x0005)
	v.OutGC(0xFF08)
	v.OutGC(0x0003)
	v.OutGC(0x0000)
	v.OutGC(0x0F01)
	for di := uint16(0); di < 0x6D60; di++ {
		if off, ok := vram(di); ok {
			v.Write(off, 0)
		}
	}
	v.OutGC(0x0000)
	src := g.buf[g.bufBase():]
	si := 0
	next := func() byte {
		var b byte
		if si < len(src) {
			b = src[si]
		}
		si++
		return b
	}
	enable := [4]uint16{0x0701, 0x0B01, 0x0D01, 0x0E01}
	for p := 0; p < 4; p++ {
		di := uint16(0)
		bh := byte(1)
		var bl byte
		v.OutGC(0x1003)
		v.OutGC(enable[p])
		for col := 0; col < 80; col++ {
			for row := 0; row < 350; row++ {
				bh--
				switch {
				case bh&0x7F == 0:
					bh = next()
					bl = next()
				case bh&0x80 == 0:
					bl = next()
				}
				if off, ok := vram(di); ok {
					v.Read(off)
					v.Write(off, bl)
				}
				di += 80
			}
			di -= 0x6D5F
		}
	}
	v.OutGC(0x0105)
	// очистка страницы и распаковка 112 000 байт: около 9 команд на байт
	g.spin(0x6D60 + 80*350*4*9)
}

// setPalette — L_4782: загрузить 16 регистров палитры из DS:si.
// Повторная загрузка той же палитры пропускается (кроме текстовой 1034h).
func (g *Game) setPalette(si uint16) {
	if si == g.w(vPalCache) && si != palText {
		g.sw(vPalCache, si)
		return
	}
	g.sw(vPalCache, si)
	var p [16]byte
	copy(p[:], g.ds[si:si+16])
	g.v.SetPalette(p)
}

// Палитры.
const (
	vPalCache = 0x1032
	palText   = 0x1034 // палитра текстового режима
	palImage  = 0x1045 // палитра из заголовка загруженной картинки
	palBlack  = 0x1056 // чёрная
)

// clearPage0 — L_4185: залить страницу 0 цветом 15 (в палитрах игры — чёрный)
// и перейти в режим записи 1.
func (g *Game) clearPage0() {
	v := g.v
	v.OutGC(0x0005)
	v.OutGC(0x0000)
	v.OutGC(0x0001)
	v.OutGC(0x0002)
	v.OutGC(0x0003)
	v.OutGC(0x0004)
	v.OutGC(0x0F07)
	v.OutGC(0xFF08)
	v.OutGC(0x0F00)
	v.OutGC(0x0F01)
	for di := uint16(0); di < 0x6D60; di++ {
		v.Write(di, 0x01)
	}
	v.OutGC(0x0001)
	v.OutGC(0x0105)
	g.spin(0x36B0 * 4)
}

// saveState — L_0bdb: скопировать DS:0.. в блок памяти seg.
func (g *Game) saveState(seg uint16) {
	n := playerSize
	if seg == segInit {
		n = initSize
	}
	b := g.saves[seg]
	if b == nil {
		b = make([]byte, n)
		g.saves[seg] = b
	}
	copy(b, g.ds[:n])
}

// restoreState — L_0bf5: вернуть DS:0.. из блока seg.
func (g *Game) restoreState(seg uint16) {
	n := playerSize
	if seg == segInit {
		n = initSize
	}
	b := g.saves[seg]
	if b == nil {
		b = make([]byte, n)
		g.saves[seg] = b
	}
	copy(g.ds[:n], b)
}

// loadCharTab — L_0068: прочитать CHARTAB.CDM в DS:254Dh.
func (g *Game) loadCharTab() {
	h, ok := g.open(fnCharTab)
	if !ok {
		g.v.SetMode(3)
		g.printDOS(vMsgFatal)
		panic(exitDOS{})
	}
	g.readFile(h, g.ds[charTabOff:charTabOff+0xDE4])
	g.closeFile(h)
}

// setTextFonts — L_0da0: шрифт CHARTAB во второй блок, 350 строк, текстовая
// палитра, яркие символы — из второго блока.
func (g *Game) setTextFonts() {
	g.v.LoadFont(1, g.ds[charTabOff:charTabOff+0xDE4], 0, 0xFE, 14)
	g.setPalette(palText)
	g.v.SelectFonts(4)
}

// textMode — «mov ax, 3; int 10h; mov cx, 2000h; mov ah, 1; int 10h;
// mov ax, 1003h; mov bl, 0; int 10h»: текстовый режим без курсора и мигания.
func (g *Game) textMode(keep bool) {
	m := byte(3)
	if keep {
		m |= 0x80
	}
	g.v.SetMode(m)
	g.v.SetCursorShape(0x2000)
	g.v.SetBlink(false)
}

// graphMode — «mov ax, 10h; int 10h».
func (g *Game) graphMode(keep bool) {
	m := byte(0x10)
	if keep {
		m |= 0x80
	}
	g.v.SetMode(m)
}

// teletype — INT 10h, AH=0Eh в текущем режиме (используется только для
// сообщений DOS).
func (g *Game) teletype(ch byte) {
	g.v.Teletype(ch, 7)
}

// ---- клавиатура ----

// Переменные клавиатуры и джойстика.
const (
	vJoystick  = 0x027D // управление джойстиком
	vJoySaved  = 0x027E
	vJoyRead   = 0x027F // последний опрос был от джойстика
	vJoyCX     = 0x0280 // центр джойстика
	vJoyCY     = 0x0282
	vLastKey   = 0x0288
	vDemo      = 0x1396 // идёт демонстрация
	vDemoTitle = 0x1397 // заставка между демонстрациями
	vDemoCount = 0x026C // сколько опросов подряд без клавиши
	vDemoKey   = 0x026E // очередное слово записи
	vDemoStop  = 0x026B // демонстрация прервана
	vSkipKey   = 0x1070 // ожидание прервано клавишей
	vInMenu    = 0x2074 // меню вызвано не из игры (Esc игнорируется)
	noKey      = 0xFFFF
	pollCost   = 12 // команд на один опрос клавиатуры (L_4853 без нажатия)
	scanCtrlQ  = 0x1011
	asciiEsc   = 0x1B
	asciiEnter = 0x0D
	scanUp     = 0x48
	scanDown   = 0x50
	scanLeft   = 0x4B
	scanRight  = 0x4D
	scanDel    = 0x53
	asciiBksp  = 0x08
	asciiTab   = 0x09
)

// keyHit — INT 16h, AH=1.
func (g *Game) keyHit() (uint16, bool) {
	if g.quitReq.Load() {
		panic(quitNow{})
	}
	g.spin(pollCost)
	return g.in.Peek()
}

// readKeyNow — INT 16h, AH=0 при уже имеющейся клавише.
func (g *Game) readKeyNow() uint16 {
	k, _ := g.in.Read()
	return k
}

// poll — L_4853: прочитать все нажатия из буфера и вернуть последнее
// (0FFFFh — нет нажатий). Во время демонстрации нажатие подменяется записью.
func (g *Game) poll() uint16 {
	ax := uint16(noKey)
	if _, ok := g.keyHit(); ok {
		for {
			ax = g.readKeyNow()
			if _, more := g.keyHit(); !more {
				break
			}
		}
	} else if g.b(vJoystick) != 0 {
		ax = g.joystick()
	}
	if g.b(vDemo) != 0 {
		ax = g.demoKey(ax)
	}
	return ax
}

// peekKey — L_4845: как poll, но нажатие остаётся в буфере (INT 16h, AH=1).
func (g *Game) peekKey() uint16 {
	g.sb(vJoyRead, 0)
	ax, ok := g.keyHit()
	if !ok {
		ax = noKey
		if g.b(vJoystick) != 0 {
			ax = g.joystick()
		}
	}
	if g.b(vDemo) != 0 {
		ax = g.demoKey(ax)
	}
	return ax
}

// getKey — L_49cd: ждать клавишу. Повтор одного и того же направления
// джойстика не считается новым нажатием.
func (g *Game) getKey() uint16 {
	for {
		ax := g.poll()
		if ax == noKey {
			g.sw(vLastKey, ax)
			continue
		}
		if g.b(vJoyRead) != 0 {
			if ax == g.w(vLastKey) {
				continue
			}
			g.sw(vLastKey, ax)
		}
		return ax
	}
}

// joystick — L_487d: опрос джойстика (порт 201h): кнопка 1 — Enter,
// кнопка 2 — Esc, отклонение — стрелка (только если два замера совпали).
func (g *Game) joystick() uint16 {
	g.sb(vJoyRead, 1)
	al := g.joyPort()
	if al&0x10 == 0 {
		return 0x0100 | asciiEnter
	}
	if al&0x20 == 0 {
		return 0x0100 | asciiEsc
	}
	a := g.joyDirection()
	if b := g.joyDirection(); a != b {
		return noKey
	}
	return a
}

// joyDirection — L_48ad: направление отклонения джойстика от центра
// (скан-код стрелки в AH) или 0FFFFh.
func (g *Game) joyDirection() uint16 {
	bx := g.joyAxis(1)
	cx := g.joyAxis(2)
	var dh, dl byte
	if bx < g.w(vJoyCX) {
		bx = g.w(vJoyCX) - bx
		dh = 2
	} else {
		bx -= g.w(vJoyCX)
	}
	if bx < 0x64 {
		bx = 0
	}
	if cx < g.w(vJoyCY) {
		cx = g.w(vJoyCY) - cx
	} else {
		cx -= g.w(vJoyCY)
		dl = 8
	}
	if cx < 0x64 {
		if bx == 0 {
			return noKey
		}
		cx = 0
	}
	if bx >= cx {
		return uint16(0x4D-dh) << 8
	}
	return uint16(0x48+dl) << 8
}

// pollDouble — L_4916: опрос клавиатуры в режиме Double (без подмены демонстрацией).
func (g *Game) pollDouble() uint16 {
	if _, ok := g.keyHit(); ok {
		var ax uint16
		for {
			ax = g.readKeyNow()
			if _, more := g.keyHit(); !more {
				return ax
			}
		}
	}
	if g.b(vJoystick) == 0 {
		return noKey
	}
	return g.joystick()
}

// ---- генератор случайных чисел ----

// Переменные генератора.
const (
	vSeed     = 0x1067
	vSeedUses = 0x1069
)

// reseed — L_49eb: засев от тиков таймера BIOS (12345 во время демонстрации).
func (g *Game) reseed() {
	g.sw(vSeed, uint16(g.biosTicks())|1)
	if g.b(vDemo) == 1 || g.fixedSeed {
		g.sw(vSeed, 0x3039)
	}
}

// random — L_4a97: seed = seed·41; каждые 100 вызовов — новый засев.
// Возвращает AX (обычно используется старший байт).
func (g *Game) random() uint16 {
	ah := g.b(vSeedUses) + 1
	if int8(ah) >= 100 {
		ah = 0
		g.reseed()
	}
	g.sb(vSeedUses, ah)
	s := g.w(vSeed) * 41
	g.sw(vSeed, s)
	return s
}

// random2 — L_4a24: два шага генератора без пересева; AL — старший байт
// первого, AH — второго.
func (g *Game) random2() uint16 {
	s := g.w(vSeed)
	s1 := s * 41
	s2 := s1 * 41
	g.sw(vSeed, s2)
	return uint16(s1>>8)&0xFF | s2&0xFF00
}

// ---- задержки ----

// Переменные скорости.
const (
	vDelay      = 0x106C // калиброванная задержка
	vDelayStep  = 0x106E // шаг изменения задержки
	vDelayMul   = 0x106A // множитель для delayN/waitN
	vDelaySaved = 0x0497
	vSpeedHi    = 0x24F2 // 7..15
	vSpeedLo    = 0x24F6 // 1..14
)

// warp — скорость «Warp»: задержки отключены.
func (g *Game) warp() bool {
	return g.w(vSpeedHi) == 7 && g.b(vSpeedLo) == 0x0E
}

// delayTick — L_2682: задержка g_106c × 8 чтений видеопамяти (5 при джойстике).
func (g *Game) delayTick() {
	if g.warp() {
		return
	}
	inner := 8.0
	if g.b(vJoystick) != 0 {
		inner = 5
	}
	g.spin(float64(g.w(vDelay)) * (2*inner + 6))
}

// delayN — L_4ac6: задержка g_106c × g_106a чтений.
func (g *Game) delayN() {
	if g.warp() {
		return
	}
	g.spin(float64(g.w(vDelay)) * (2*float64(g.w(vDelayMul)) + 4))
}

// waitN — L_4aee: задержка g_106c × g_106a с проверкой клавиатуры на каждом
// шаге; нажатие прерывает ожидание (g_1070=1), Esc вызывает меню.
// При «Warp» лишь проверяется наличие клавиши.
func (g *Game) waitN() uint16 {
	return g.wait(true)
}

// waitNKey — L_4b6f: как waitN, но клавиша не извлекается из буфера целиком.
func (g *Game) waitNKey() uint16 {
	return g.wait(false)
}

func (g *Game) wait(flush bool) uint16 {
	g.sb(vSkipKey, 0)
	if g.warp() {
		if _, ok := g.keyHit(); ok {
			g.sb(vSkipKey, 1)
			return g.readKeyNow()
		}
		return noKey
	}
	bx := g.w(vDelay)
	if g.b(vJoystick) != 0 {
		bx = bx>>5 + 1
	}
	step := 2*float64(g.w(vDelayMul)) + 8
	for n := int(bx); n > 0; n-- {
		g.spin(step)
		if g.b(vDemo) != 0 {
			if ax, ok := g.keyHit(); ok {
				g.sb(vSkipKey, 1)
				return ax
			}
			continue
		}
		var ax uint16
		if flush {
			ax = g.poll()
		} else {
			ax = g.peekKey()
		}
		if ax == noKey {
			continue
		}
		if !flush && byte(ax) != asciiEsc {
			g.sb(vSkipKey, 1)
			return ax
		}
		if byte(ax) != asciiEsc {
			continue
		}
		if g.b(vInMenu) == 0 {
			g.escMenu()
			return ax
		}
		g.sb(vSkipKey, 1)
		return ax
	}
	return noKey
}
