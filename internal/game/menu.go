package game

// Переменные меню (колонки SEE, SPEED, GAME, PLAYER, SOUND; bx = 2 × номер колонки).
const (
	vColText   = 0x2079 // тексты колонок
	vColPos    = 0x2083 // положение колонки на экране
	vColSel    = 0x208D // выбранная строка × 140h
	vColMax    = 0x2097 // последняя строка × 140h
	vColRows   = 0x20A1 // строк в колонке (с заголовком)
	vColDescr  = 0x20AB // номер подсказки × 4
	vHotkeys   = 0x20BF // буквы пунктов, по 5 на колонку
	vBoxSingle = 0x21BE // символы рамки обычной колонки
	vBoxDouble = 0x21E9 // и текущей
	vDescrGame = 0x2214 // подсказки во время игры
	vDescrMenu = 0x221E // подсказки без игры
	vDivisors  = 0x1023 // 10000, 1000, 100, 10
	vMenuDone  = 0x2075 // выйти из меню
	vPosition  = 0x2076 // показать текущую позицию
	vStep      = 0x0297 // шаг перехода между колонками (±2)
	vSpeedPtr  = 0x24F4
	selGame    = vColSel + 4
	descrGame  = vColDescr + 4
)

// clearText — L_0d77: очистить текстовый экран (при g_2077 — со строки 5).
func (g *Game) clearText() {
	di, n := uint16(0), 0x7D0
	if g.b(vAfterGame) == 1 {
		di, n = 0x320, 0x640
	}
	for i := 0; i < n; i++ {
		g.v.TextW(di, 0x0020)
		di += 2
	}
}

// logos — L_0dcd: логотипы символами CHARTAB в строках 0..3.
func (g *Game) logos() {
	if g.b(vAfterGame) != 1 {
		for di := uint16(0); di < 0x320; di += 2 {
			g.v.TextW(di, 0x0020)
		}
	}
	block := func(di uint16, ch byte, n int, attrs uint16) {
		for r := uint16(0); r < 4; r++ {
			ah := uint16(g.b(attrs+r)) << 8
			for i := 0; i < n; i++ {
				g.v.TextW(di, ah|uint16(ch))
				di += 2
				ch++
			}
			di += 160 - uint16(n)*2
		}
	}
	block(0x0A, 0x0F, 14, 0x2549)
	block(0x2E, 0x7A, 33, 0x2545)
	block(0x78, 0x0F, 14, 0x2549)
}

// printStr — L_237f: вывести строку DS:si атрибутом ah с позиции di до 0FFh;
// 0Dh — перевод строки. Возвращает позицию после строки.
func (g *Game) printStr(si, di uint16, ah byte) uint16 {
	start := di
	for {
		al := g.b(si)
		si++
		switch al {
		case 0xFF:
			g.sw(vStep, start)
			return di
		case 0x0D:
			start += 0xA0
			di = start
		default:
			g.v.TextW(di, uint16(ah)<<8|uint16(al))
			di += 2
		}
	}
}

// mainMenu — L_0e5c: меню. Возвращается, когда выбрано продолжение игры
// или показ позиции; начало новой игры и выход уходят на L_0127.
func (g *Game) mainMenu() {
	g.sb(vStop, 0)
	g.sb(vMenuDone, 0)
	g.sb(vPosition, 0)
	g.sb(vAfterGame, 0)
	g.clearText()
redraw:
	g.logos()
	g.sw(selGame, 0x140)
	g.sw(descrGame, 0)
	if g.b(vInGame) == 1 {
		g.sw(selGame, 0x280)
		g.sw(descrGame, 4)
	}
	for bx := uint16(0); bx < 10; bx += 2 {
		ah := byte(7)
		if bx == 4 {
			ah = 4
		}
		g.drawBox(bx, ah)
		si, di := g.w(vColText+bx), g.w(vColPos+bx)
		dx := uint16(0)
		for n := g.w(vColRows + bx); n > 0; n-- {
			ah := byte(4)
			if bx == 4 {
				ah = 1
			}
			if dx != 0 && dx != g.w(vColSel+bx) {
				ah = 7
			}
			for i := 0; i < 10; i++ {
				g.v.TextW(di, uint16(ah)<<8|uint16(g.b(si)))
				si++
				di += 2
			}
			di += 0x12C
			dx += 0x140
		}
	}
	g.showSpeed(4)
	bx := uint16(4)
	if g.b(vDemo) == 1 {
		bx = 0
	}
	for {
		g.describe(bx)
	key:
		ax := g.getKey()
		al, ah := byte(ax), byte(ax>>8)
		if al != 0 {
			if al == asciiEnter {
				bx = g.menuEnter(bx)
				if g.b(vNoHelp) == 1 {
					g.sb(vNoHelp, 0)
					g.sb(vAfterGame, 0)
					goto key
				}
				if g.b(vAfterGame) == 1 {
					g.sb(vAfterGame, 0)
					goto redraw
				}
				if g.b(vMenuDone) == 1 {
					return
				}
				continue
			}
			if ax == scanCtrlQ {
				g.exitNow()
			}
			g.menuLetter(bx, al)
			continue
		}
		switch ah {
		case scanUp:
			if bx == 2 {
				if !g.speedUp() {
					goto key
				}
				if !(g.w(vSpeedHi)&1 == 0 && g.b(vSpeedLo) == 8) {
					goto key
				}
			}
			g.menuMove(bx, true)
		case scanDown:
			if bx == 2 {
				if !g.speedDown() {
					goto key
				}
				if !(g.w(vSpeedHi)&1 == 0 && g.b(vSpeedLo) == 7) {
					goto key
				}
			}
			g.menuMove(bx, false)
		case scanLeft:
			if bx == 0 {
				continue
			}
			g.sw(vStep, 0xFFFE)
			bx = g.menuColumn(bx)
		case scanRight:
			if bx == 8 {
				continue
			}
			g.sw(vStep, 2)
			bx = g.menuColumn(bx)
		}
	}
}

// exitNow — L_4308: немедленный выход в DOS (Ctrl+Q).
func (g *Game) exitNow() {
	g.speakerOff()
	g.v.SetMode(3)
	panic(exitDOS{})
}

// speedUp — стрелка вверх в колонке SPEED: быстрее (меньше задержка).
// false — скорость уже наибольшая.
func (g *Game) speedUp() bool {
	if g.b(vSpeedLo) < 0x0E {
		g.sb(vSpeedLo, g.b(vSpeedLo)+1)
	} else {
		if g.w(vSpeedHi) <= 7 {
			return false
		}
		g.sb(vSpeedLo, 1)
		g.decw(vSpeedHi)
	}
	g.sw(vDelay, g.w(vDelay)-g.w(vDelayStep))
	g.showSpeed(1)
	return true
}

// speedDown — стрелка вниз в колонке SPEED: медленнее.
func (g *Game) speedDown() bool {
	if g.b(vSpeedLo) > 1 {
		g.sb(vSpeedLo, g.b(vSpeedLo)-1)
		g.sw(vDelay, g.w(vDelay)+g.w(vDelayStep))
	} else {
		if g.w(vSpeedHi) >= 0x0F {
			return false
		}
		g.sb(vSpeedLo, 0x0E)
		g.sw(vDelay, g.w(vDelay)+g.w(vDelayStep))
		g.incw(vSpeedHi)
	}
	g.showSpeed(1)
	return true
}

// setRowAttr задаёт атрибут n символам строки колонки bx со смещением sel.
func (g *Game) setRowAttr(bx, sel uint16, n int, dl byte) {
	di := g.w(vColPos+bx) + sel
	for i := 0; i < n; i++ {
		g.v.TextB(di+1, dl)
		di += 2
	}
}

// menuMove — L_0f69 / L_1006: переместить выбор в колонке вверх или вниз.
func (g *Game) menuMove(bx uint16, up bool) {
	ax := g.w(vColSel + bx)
	n := 9
	if up && bx != 2 {
		n = 10
	}
	g.setRowAttr(bx, ax, n, 7)
	if up {
		ax -= 0x140
		g.sw(vColDescr+bx, g.w(vColDescr+bx)-4)
		if int16(ax) < 0x140 {
			g.addw(vColDescr+bx, 4)
			ax += 0x140
		}
	} else {
		ax += 0x140
		g.addw(vColDescr+bx, 4)
		if int16(ax) > int16(g.w(vColMax+bx)) {
			ax -= 0x140
			g.sw(vColDescr+bx, g.w(vColDescr+bx)-4)
		}
	}
	g.sw(vColSel+bx, ax)
	g.setRowAttr(bx, ax, n, 1)
}

// menuColumn — L_1061: перейти в соседнюю колонку (шаг g_0297).
func (g *Game) menuColumn(bx uint16) uint16 {
	dl := byte(4)
	g.drawBox(bx, 7)
	if bx == 2 {
		g.showSpeed(4)
	}
	for {
		di := g.w(vColPos + bx)
		bp := g.w(vColSel + bx)
		for i := 0; i < 9; i++ {
			g.v.TextB(di+1, dl)
			g.v.TextB(bp+di+1, dl)
			di += 2
		}
		if dl == 1 {
			return bx
		}
		dl = 1
		bx += g.w(vStep)
		g.drawBox(bx, 4)
		if bx == 2 {
			g.showSpeed(1)
		}
	}
}

// menuLetter — L_10e8: буква пункта выбирает его в текущей колонке.
func (g *Game) menuLetter(bx uint16, al byte) {
	if int8(al) >= 0x61 {
		al -= 0x20
	}
	si := bx>>1 + bx + bx
	dx, bp := uint16(0), uint16(0x140)
	found := false
	for i := 0; i < 5; i++ {
		if al == g.b(vHotkeys+si) {
			found = true
			break
		}
		si++
		dx += 4
		bp += 0x140
	}
	if !found || bx == 2 {
		return
	}
	g.sw(vColDescr+bx, dx)
	bp = g.toggleItem(bx, al, bp)
	g.setRowAttr(bx, g.w(vColSel+bx), 9, 7)
	g.sw(vColSel+bx, bp)
	g.setRowAttr(bx, bp, 9, 1)
}

// toggleItem — L_1150: повторная буква переключает Continue/Control
// в колонке GAME и On/Off в колонке SOUND.
func (g *Game) toggleItem(bx uint16, al byte, bp uint16) uint16 {
	switch bx {
	case 4:
		if al != 'C' {
			return bp
		}
		switch g.w(vColSel + bx) {
		case 0x280:
			g.sw(vColDescr+bx, 8)
			return 0x3C0
		case 0x3C0:
			g.sw(vColDescr+bx, 4)
			return 0x280
		}
	case 8:
		switch g.w(vColSel + bx) {
		case 0x140:
			g.sw(vColDescr+bx, 4)
			return 0x280
		case 0x280:
			g.sw(vColDescr+bx, 0)
			return 0x140
		}
	}
	return bp
}

// menuEnter — L_1341: Enter выполняет пункт колонки SEE или, в любой другой
// колонке, выбранный пункт колонки GAME. Возвращает новое значение BX.
func (g *Game) menuEnter(bx uint16) uint16 {
	if bx != 0 {
		dx := bx
		switch g.w(selGame) {
		case 0x140: // Start
			g.applySettings()
			g.sb(vInGame, 0)
			g.sb(vNewGame, 1)
			panic(toMain{})
		case 0x280: // Continue
			if g.b(vInGame) != 1 {
				return dx
			}
			g.sb(vMenuDone, 1)
			g.applySettings()
			return 8
		case 0x3C0: // Control
			g.controlMenu()
			return 4
		default: // Quit
			g.sw(vDots, 0)
			g.sb(vStop, 1)
			g.sb(vQuit, 1)
			g.sb(vMenuDone, 1)
			panic(toMain{})
		}
	}
	switch g.w(vColSel) {
	case 0x140: // Top-10
		g.sb(vAfterGame, 1)
		g.clearText()
		g.showTables(4)
		g.clearText()
		g.sb(vAfterGame, 1)
	case 0x280: // Info
		g.help()
		if g.b(vNoHelp) != 1 {
			g.clearText()
		}
		g.sb(vAfterGame, 1)
	case 0x3C0: // Demo
		g.startDemo()
	default: // Position
		if g.b(vInGame) == 1 {
			g.sb(vPosition, 1)
			g.sb(vMenuDone, 1)
		}
	}
	return bx
}

// applySettings — L_13fe: применить выбор колонок PLAYER и SOUND.
func (g *Game) applySettings() {
	g.sb(vPlayersSet, 1)
	g.sb(vDoubleSet, 0)
	g.sb(vComputer, 0)
	switch g.w(vColSel + 6) {
	case 0x140:
	case 0x280:
		g.sb(vPlayersSet, 2)
	case 0x3C0:
		g.sb(vDoubleSet, 1)
	default:
		g.sb(vComputer, 1)
	}
	g.sb(vSoundOn, 1)
	if g.w(vColSel+8) != 0x140 {
		g.sb(vSoundOn, 0)
	}
}

// drawBox — L_1453: рамка колонки bx (двойная для текущей, ah=4).
func (g *Game) drawBox(bx uint16, ah byte) {
	si := uint16(vBoxSingle)
	if ah == 4 {
		si = vBoxDouble
	}
	at := uint16(ah) << 8
	di := g.w(vColPos+bx) - 0xA4
	line := func() {
		for i := 0; i < 14; i++ {
			g.v.TextW(di, at|uint16(g.b(si)))
			si++
			di += 2
		}
	}
	line()
	di += 0x84
	bar := at | uint16(g.b(si+0x1C))
	g.v.TextW(di, bar)
	di += 0x1A
	g.v.TextW(di, bar)
	di += 0x86
	line()
	bar = at | uint16(g.b(si+0x0E))
	cx := (g.w(vColRows+bx)-1)*2 - 1
	di -= 2
	for ; cx > 0; cx-- {
		di += 0x86
		g.v.TextW(di, bar)
		di += 0x1A
		g.v.TextW(di, bar)
	}
	di += 0x86
	line()
}

// showSpeed — L_11a7: число скорости и полоска (символы CHARTAB 0..0Eh).
func (g *Game) showSpeed(ah byte) {
	dx := uint16(g.w(vSpeedHi))
	dl := byte(dx)
	dl = -dl + 0x0F
	dl <<= 1
	bl := dl
	dl <<= 3
	dl -= bl
	dl += g.b(vSpeedLo)
	dx = dx&0xFF00 | uint16(dl)
	di := uint16(0x3F4)
	at := uint16(ah) << 8
	bx := uint16(0x1027)
	for {
		al := byte(0)
		for int16(dx) >= int16(g.w(bx)) {
			al++
			dx -= g.w(bx)
		}
		g.v.TextW(di, at|uint16(al+'0'))
		di += 2
		bx += 2
		if g.b(bx-2) == 0x0A {
			break
		}
	}
	g.v.TextW(di, at|uint16(byte(dx)+'0'))
	ah += 4
	if ah < 8 {
		ah += 4
	}
	at = uint16(ah) << 8
	g.sw(vSpeedPtr, 0)
	if g.w(vSpeedHi) > 0x0F {
		g.sw(vSpeedHi, 0x0F)
	}
	if int16(g.w(vSpeedHi)) < 7 {
		g.sw(vSpeedHi, 7)
	}
	if int16(g.w(vSpeedHi)) > 7 {
		di = 0x538
		for cx := g.w(vSpeedHi) - 7; cx > 0; cx-- {
			g.v.TextW(di, at)
			g.v.TextW(di+2, at)
			di += 0xA0
		}
	}
	g.sw(vSpeedPtr, 0)
	for cx := g.w(vSpeedHi) + 1; cx > 0; cx-- {
		g.addw(vSpeedPtr, 0xA0)
	}
	g.addw(vSpeedPtr, 0x38)
	di = g.w(vSpeedPtr)
	g.v.TextW(di, at|uint16(g.b(vSpeedLo)))
	g.v.TextW(di+2, at|uint16(g.b(vSpeedLo)))
	if g.w(vSpeedHi) < 0x0F {
		g.addw(vSpeedPtr, 0xA0)
		di = g.w(vSpeedPtr)
		for cx := 0x0F - g.w(vSpeedHi); cx > 0; cx-- {
			g.v.TextW(di, at|0x0E)
			g.v.TextW(di+2, at|0x0E)
			di += 0xA0
		}
	}
}

// describe — L_1299: подсказка к выбранному пункту в рамке внизу экрана.
func (g *Game) describe(bx uint16) {
	for di := uint16(0xD20); di < 0xD20+0xF0*2; di += 2 {
		g.v.TextW(di, 0x0420)
	}
	tab := g.w(vDescrMenu + bx)
	if g.b(vInGame) == 1 {
		tab = g.w(vDescrGame + bx)
	}
	si := tab + g.w(vColDescr+bx)
	al := g.b(g.w(si))
	if second := g.w(si + 2); second != 0 {
		al += g.b(second)
	}
	g.framedText(uint16(al), si, 0xE10)
}

// message — L_1cf8: сообщение в рамке внизу экрана. DS:si — длина и два
// указателя на части текста; сообщения начиная с 1321h выводятся ниже.
func (g *Game) message(si uint16) {
	di, bp := uint16(0xDC0), uint16(0xEB0)
	if si < 0x1321 {
		di, bp = 0xD20, 0xE10
	}
	for i := 0; i < 0xF0; i++ {
		g.v.TextW(di, 0x0420)
		di += 2
	}
	// L_1d1d
	g.framedText(uint16(g.b(si)), si+2, bp)
}

// framedText — L_12d5: текст длиной n из двух частей (указатели DS:ptrs и
// DS:ptrs+2, у каждой первый байт — длина) по центру позиции di в рамке.
func (g *Game) framedText(n, ptrs, di uint16) {
	first, second := g.w(ptrs), g.w(ptrs+2)
	di = (di - n) &^ 1
	start := di
	bp := n << 1
	di -= 0xA2
	put := func(off uint16, ch byte) { g.v.TextB(off, ch) }
	put(di-2, 0xDA)
	put(di+0x9E, 0xB3)
	put(di+0x13E, 0xC0)
	put(bp+di+4, 0xBF)
	put(bp+di+0xA4, 0xB3)
	put(bp+di+0x144, 0xD9)
	bp = bp>>1 + 2
	for k := 0; k < 2; k++ {
		for i := uint16(0); i < bp; i++ {
			put(di, 0xC4)
			di += 2
		}
		di += 0x140 - 2*bp
	}
	di = g.printStr(first+1, start, 1)
	if second != 0 {
		g.printStr(second+1, di, 1)
	}
}
