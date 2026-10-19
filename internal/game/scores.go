package game

// Таблицы рекордов и статистики хранятся текстом, готовым к выводу
// (DS:165Bh..1F07h, это же содержимое HIGHSC.CDM).
const (
	vTopScores  = 0x1985 // десять лучших результатов (слова)
	vNewRow     = 0x1980 // 1 — результат не попал в десятку
	vNewPos     = 0x1981 // место последнего игрока
	vLastSel    = 0x1983 // строка «LAST PLAYER»: 1 — одного игрока, 2 — одна из двух
	vLastWhich  = 0x1984 // в игре на двоих: 1 — строка первого игрока
	vEntryRow   = 0x206D // строка экрана новой записи
	vEntryPos   = 0x206B // её смещение в текстовой памяти
	vEntryName  = 0x2069 // поле имени новой записи (DS)
	vFieldStart = 0x206E // столбцы начала полей имени, даты и времени
	vFieldEnd   = 0x2071 // и их конца
	vPaletteCyc = 0x24F1 // перелив рамки таблиц
	vTimerOn    = 0x0251 // учёт игрового времени: идёт
	vTimerH     = 0x0254 // момент начала
	vTimerM     = 0x0255
	vTimerS     = 0x0256
	vStats1     = 0x0259 // статистика игрока 1: ходы, монстры, бонусы, время ч:м:с
	vStats2     = 0x0262 // и игрока 2
	vCurPlayer  = 0x023B // чей ход в игре на двоих (1, 2)
	vTurn       = 0x0223 // номер игрока (0, 1)
	vTurn2      = 0x0225 // он же × 2
	vScore      = 0x0233 // очки игроков (слова)
	msgEnter1   = 0x1F0B
	msgEnterP1  = 0x1F2F
	msgEnterP2  = 0x1F5E
	msgToStats  = 0x1FE9
	msgToTop    = 0x202B
)

// afterGame — L_1a04: после игры — ввод имени, если результат попал в
// десятку, иначе результат записывается в строку «LAST PLAYER»; затем таблицы.
func (g *Game) afterGame() {
	g.sb(vAfterGame, 0)
	g.sb(vEntryRow, 8)
	g.sw(vEntryPos, 0x500)
	ax := g.w(vScore + g.w(vTurn2))
	for bx := uint16(0); bx != 0x14; bx += 2 {
		if int16(ax) > int16(g.w(vTopScores+bx)) {
			// L_1aec
			g.sb(vNewRow, 0)
			g.insertTop(bx, ax)
			g.insertStats(bx)
			g.clearText()
			g.showTables(4)
			return
		}
		g.sb(vEntryRow, g.b(vEntryRow)+1)
		g.addw(vEntryPos, 0xA0)
	}
	var di, bp uint16
	switch {
	case g.b(vDouble) == 1:
		g.sb(vLastSel, 2)
		if g.b(vLastWhich) != 0 {
			g.sb(0x1E2F, ' ')
			g.sb(0x1E30, ' ')
			di, bp = 0x1E5A, 0x18D0
		} else {
			g.sb(0x1E79, ' ')
			g.sb(0x1E7A, ' ')
			di, bp = 0x1EA4, 0x1905
		}
	case g.b(vPlayers) != 2:
		g.sb(vLastSel, 1)
		g.sb(0x1DE5, ' ')
		g.sb(0x1DE6, ' ')
		di, bp = 0x1E10, 0x189B
	default:
		g.sb(vLastSel, 2)
		if g.b(vCurPlayer) != 2 {
			g.sb(vLastWhich, 1)
			g.sb(0x1E2F, ' ')
			g.sb(0x1E30, ' ')
			di, bp = 0x1E5A, 0x18D0
		} else {
			g.sb(vLastWhich, 0)
			g.sb(0x1E79, ' ')
			g.sb(0x1E7A, ' ')
			di, bp = 0x1EA4, 0x1905
		}
	}
	g.sb(vNewRow, 1)
	g.sw(vNewPos, 0x64)
	g.formatEntry(di, bp, ax, 0)
	g.showTables(4)
	g.poll()
}

// insertTop — L_1d29: вставить результат ax на место bx/2 таблицы десяти
// лучших и ввести имя.
func (g *Game) insertTop(bx, ax uint16) {
	// сдвиг очков
	n := 10 - bx>>1
	si := uint16(vTopScores + 0x12)
	for ; n > 0; n-- {
		g.sw(si+2, g.w(si))
		si -= 2
	}
	g.sw(si+2, ax)
	// сдвиг строк таблицы (по 0x49 байт, 0x3F байт от поля имени)
	si = 0x1D0F
	di := si + 0x49
	for n = 9 - bx>>1; n > 0; n-- {
		copy(g.ds[di:di+0x3F], g.ds[si:si+0x3F])
		si -= 0x49
		di -= 0x49
	}
	g.sw(vEntryName, di)
	for i := uint16(0); i < 0x22; i++ {
		g.sb(di+i, ' ')
	}
	di += 0x22 + 3
	g.formatEntry(di, 0, ax, bx)
}

// formatEntry — L_1d79: записать в строку таблицы очки ax, дату и время;
// для новой записи в десятке — ввести имя (bx — её место × 2).
func (g *Game) formatEntry(di, bp, ax, bx uint16) {
	dx := ax
	for d := uint16(vDivisors); ; d += 2 {
		al := byte(0)
		for int16(dx) >= int16(g.w(d)) {
			al++
			dx -= g.w(d)
		}
		g.sb(di, al+'0')
		di++
		if g.b(d) == 0x0A {
			break
		}
	}
	g.sb(di, byte(dx)+'0')
	di += 5
	year, mon, day := g.dosDate()
	cx := year - 0x708
	for {
		cx -= 0x64
		if cx < 0x64 {
			break
		}
	}
	di = g.digits2(di, byte(cx))
	di = g.digits2(di, mon)
	di = g.digits2(di, day)
	hh, mm, _, _ := g.dosTime()
	di += 2
	di = g.digits2(di, hh)
	g.digits2(di, mm)
	if g.b(vNewRow) == 1 {
		g.formatStats(bp)
		return
	}
	g.enterName(bx)
}

// enterName — L_1dcf: таблица с выделенной новой строкой и редактор имени,
// даты и времени (Tab — следующее поле, Backspace/Del — стереть).
func (g *Game) enterName(bx uint16) {
	g.call(0x1DCF)
	g.drawTables(7)
	di := g.w(vEntryPos)
	g.v.TextB(di+0x0D, 5)
	g.v.TextB(di+0x0F, 5)
	di += 0x19
	attrs := func(n int, a byte) {
		for i := 0; i < n; i++ {
			g.v.TextB(di, a)
			di += 2
		}
	}
	attrs(0x22, 4)
	di += 6
	attrs(6, 5)
	di += 6
	attrs(8, 4)
	di += 6
	attrs(5, 4)
	msg := uint16(msgEnter1)
	if g.b(vPlayers) != 1 {
		msg = msgEnterP1
		if g.b(vCurPlayer) != 1 {
			msg = msgEnterP2
		}
	}
	g.message(msg)
	si := uint16(0)
	di = g.w(vEntryName)
	dh, dl := g.b(vEntryRow), byte(0x0C)
	cursor := func() { g.v.SetCursor(uint16(dh)<<8 | uint16(dl)) }
	cursor()
	g.v.SetCursorShape(0x0607)
	sep := func(c byte) bool { return c == 0x3C || c == 0x3F || c == 0x47 }
	for {
		ax := g.getKey()
		al, ah := byte(ax), byte(ax>>8)
		switch {
		case al == asciiEnter:
			g.finishEntry(bx)
			return
		case al == asciiTab:
			si++
			if si == 3 {
				si = 0
			}
			dl = g.b(vFieldStart + si)
			cursor()
			di = g.w(vEntryName) + uint16(dl) - 0x0C
		case al == asciiBksp || (al == 0 && ah == scanDel):
			al = ' '
			if dl != g.b(vFieldStart+si) && !(dl == g.b(vFieldEnd+si) && g.b(di) != ' ') {
				for {
					di--
					dl--
					if !sep(dl) {
						break
					}
				}
				cursor()
			}
			g.sb(di, al)
			g.v.WriteChar(al, 4, 1)
		case al != 0:
			g.sb(di, al)
			g.v.WriteChar(al, 4, 1)
			if dl == g.b(vFieldEnd+si) {
				continue
			}
			for {
				di++
				dl++
				if !sep(dl) {
					break
				}
			}
			cursor()
		}
	}
}

// finishEntry — L_1ebe: скопировать новую запись в строку «LAST PLAYER»
// и записать места.
func (g *Game) finishEntry(bx uint16) {
	g.v.SetCursorShape(0x2000)
	si := g.w(vEntryName) + 0x25
	g.sb(vLastSel, 1)
	di := uint16(0x1E10)
	switch {
	case g.b(vPlayers) == 1:
	case g.b(vDouble) == 1:
		g.sb(vLastSel, 2)
		di = 0x1E5A
		if g.b(vLastWhich) != 1 {
			di = 0x1EA4
		}
	default:
		g.sb(vLastWhich, 0)
		g.sb(vLastSel, 2)
		di = 0x1EA4
		if g.b(vCurPlayer) != 2 {
			g.sb(vLastWhich, 1)
			di = 0x1E5A
		}
	}
	di = g.copyOverlap(si, di, 0x19)
	dx := bx>>1 + 1
	di -= 0x44
	if g.b(vCurPlayer) != 1 && dx <= g.w(vNewPos) {
		if g.w(vNewPos) <= 0x0A {
			g.number(0x1E2F, g.w(vNewPos)+1, 0x1029)
		} else {
			g.sb(0x1E2F, ' ')
			g.sb(0x1E30, ' ')
		}
	}
	g.sw(vNewPos, dx)
	g.number(di, dx, 0x1029)
}

// copyOverlap — «mov ax, [si]; mov [di], ax; inc si; inc di; loop»: копия
// n байт словами с шагом в байт (последним пишется ещё один байт).
// Возвращает di после копирования.
func (g *Game) copyOverlap(si, di uint16, n int) uint16 {
	for ; n > 0; n-- {
		g.sw(di, g.w(si))
		si++
		di++
	}
	return di
}

// number — L_234c: число dx цифрами по таблице делителей bx с ведущими
// пробелами (последняя цифра — всегда). Возвращает позицию последней цифры.
func (g *Game) number(di, dx, bx uint16) uint16 {
	seen := false
	for {
		al := byte(0)
		for int16(dx) >= int16(g.w(bx)) {
			al++
			dx -= g.w(bx)
		}
		al += '0'
		if al != '0' {
			seen = true
		} else if !seen {
			al = ' '
		}
		g.sb(di, al)
		di++
		bx += 2
		if g.b(bx-2) == 0x0A {
			break
		}
	}
	g.sb(di, byte(dx)+'0')
	return di
}

// insertStats — L_2082: сдвинуть строки статистики десятки и заполнить новую.
func (g *Game) insertStats(bx uint16) {
	si := uint16(0x17FE)
	di := si + 0x34
	for n := 9 - bx>>1; n > 0; n-- {
		copy(g.ds[di:di+0x2E], g.ds[si:si+0x2E])
		si -= 0x34
		di -= 0x34
	}
	g.formatStats(di)
}

// formatStats — L_20b3: строка статистики в DS:bp — время игры, этап и круг,
// средняя скорость, ходы, съеденные монстры и бонусы; затем она копируется в
// строку последнего игрока и время прибавляется к общему активному времени.
func (g *Game) formatStats(bp uint16) {
	si := uint16(vStats1)
	if g.b(vCurPlayer) == 2 || g.w(vTurn) == 1 {
		si = vStats2
	}
	di := g.digits2(bp, g.b(si+6))
	di = g.digits2(di, g.b(si+7))
	g.digits2(di, g.b(si+8))
	lvl := uint16(vScreen)
	if g.b(vPlayers) != 1 && g.b(vDouble) != 1 {
		lvl = 0x023C
		if g.b(vCurPlayer) != 1 {
			lvl = 0x0240
		}
	}
	di = g.number(bp+0x0B, g.w(lvl)>>1+1, 0x1029)
	g.number(di+3, g.w(lvl+2), 0x1029)
	h := g.b(si + 6)
	if h > 0x11 {
		h = 0x11
	}
	secs := uint16(h)*0x0E10 + uint16(g.b(si+7))*0x3C + uint16(g.b(si+8))
	moves := uint32(g.w(si)) << 8
	var avg uint16
	if secs != 0 && moves/uint32(secs) <= 0xFFFF {
		avg = uint16(moves / uint32(secs))
	} // в оригинале при 0 секунд — деление на ноль
	g.number(bp+0x15, avg, vDivisors)
	g.number(bp+0x1E, g.w(si), vDivisors)
	dx := g.w(si + 2)
	if dx > 0x3E7 {
		dx = 0x3E7
	}
	di = g.number(bp+0x26, dx, 0x1027)
	dx = g.w(si + 4)
	if dx > 0x63 {
		dx = 0x63
	}
	g.number(di+4, dx, 0x1029)
	if g.b(vNewRow) != 1 {
		src := bp
		g.sb(vLastSel, 1)
		dst := uint16(0x189B)
		switch {
		case g.b(vPlayers) == 1:
		case g.b(vDouble) == 1:
			g.sb(vLastSel, 2)
			dst = 0x18D0
			if g.b(vLastWhich) != 1 {
				dst = 0x1905
			}
		default:
			g.sb(vLastWhich, 0)
			g.sb(vLastSel, 2)
			dst = 0x1905
			if g.b(vCurPlayer) != 2 {
				g.sb(vLastWhich, 1)
				dst = 0x18D0
			}
		}
		g.copyOverlap(src, dst, 0x2F)
	}
	si = 0x025F
	if !(g.b(vDouble) == 1 && g.b(vLastWhich) == 0) && g.b(vCurPlayer) != 1 {
		si = 0x0268
	}
	g.addTime(0x1976, si)
}

// drawTables — L_1b3c: таблица десяти лучших атрибутом ah.
func (g *Game) drawTables(ah byte) {
	g.logos()
	di := g.printStr(0x199B, 0x288, ah)
	si := uint16(0x1DE3)
	if g.b(vLastSel) != 1 {
		si = 0x1E2D
		if g.b(vLastWhich) != 1 {
			di = g.printStr(si, di, ah)
			si = 0x1E77
		}
	}
	di = g.printStr(si, di, ah)
	g.printStr(0x1EC1, di, ah)
	g.vertical(0x24DD)
}

// vertical — L_1cde: десять символов DS:si столбиком у левого и правого края.
func (g *Game) vertical(si uint16) {
	di := uint16(0x5A2)
	for i := 0; i < 10; i++ {
		w := 0x0200 | uint16(g.b(si))
		g.v.TextW(di, w)
		g.v.TextW(di+0x9A, w)
		si++
		di += 0xA0
	}
}

// showTables — L_1b78: таблица десяти лучших; пробел переключает на
// статистику и обратно, другая клавиша — выход.
func (g *Game) showTables(attr byte) {
	for {
		g.drawTables(attr)
		g.sb(vAfterGame, 1)
		g.message(msgToStats)
		if al := byte(g.waitColor()); al == 0 || al != ' ' {
			return
		}
		g.addSessionTime()
		g.formatTotals()
		di := g.printStr(0x1412, 8, attr)
		_ = di
		di = g.printStr(0x165B, 0x532, attr)
		si := uint16(0x1898)
		if g.b(vLastSel) != 1 {
			si = 0x18CD
			if g.b(vLastWhich) != 1 {
				di = g.printStr(si, di, attr)
				si = 0x1902
			}
		}
		di = g.printStr(si, di, attr)
		g.printStr(0x1937, di, attr)
		g.vertical(0x24E7)
		g.message(msgToTop)
		al := byte(g.waitColor())
		g.sb(vAfterGame, 0)
		if al == 0 || al != ' ' {
			return
		}
	}
}

// waitColor — L_1bfd: ждать клавишу, переливая цвет рамки таблиц.
func (g *Game) waitColor() uint16 {
	for {
		g.call(0x1BFD)
		c := (g.b(vPaletteCyc) + 1) & 0x3F
		c |= 0x38
		if c == 0x38 {
			c++
		}
		g.sb(vPaletteCyc, c)
		g.v.SetPaletteReg(2, c)
		saved := g.w(vDelay)
		g.sw(vDelay, g.w(vDelaySaved))
		g.sw(vDelayMul, 0x1F4)
		ax := g.waitNKey()
		g.sw(vDelay, saved)
		if g.b(vSkipKey) == 0 {
			continue
		}
		if g.b(vJoyRead) == 0 {
			return g.poll()
		}
		switch byte(ax) {
		case asciiEnter:
			return ax
		case asciiEsc:
			return ax&0xFF00 | ' '
		}
	}
}

// formatTotals — L_1c5b: общее и активное время игры словами.
func (g *Game) formatTotals() {
	g.formatTotal(0x196C, 0x1472)
	g.formatTotal(0x1976, 0x14BB)
}

// formatTotal — L_1c74: недели, дни, часы и минуты счётчика bp в текст DS:di
// (нулевые старшие единицы пропускаются, слова — в единственном или
// множественном числе).
func (g *Game) formatTotal(bp, di uint16) {
	si := uint16(0)
	g.sb(0x13A3, 0)
	for bx := uint16(0); bx != 8; bx += 2 {
		cx := g.w(0x13A4 + bx)
		dx := g.w(bp + bx)
		if dx == 0 && g.b(0x13A3) != 1 {
			si += g.w(0x13A4 + bx)
			for ; cx > 0; cx-- {
				g.sb(di, ' ')
				di++
			}
			continue
		}
		g.sb(0x13A3, 1)
		di = g.number(di, dx, vDivisors+g.w(0x13B4+bx))
		cx -= g.w(0x13AC + bx)
		si += g.w(0x13AC + bx)
		di++
		for ; cx > 0; cx-- {
			al := g.b(0x13E7 + si)
			if dx == 1 {
				al = g.b(0x13BC + si)
			}
			g.sb(di, al)
			si++
			di++
		}
	}
}

// winner — L_1b00: экран «Welcome, master», если побит лучший результат
// (и он не меньше 1000).
func (g *Game) winner() {
	ax := g.w(vScore + g.w(vTurn2))
	if ax < g.w(vTopScores) || ax < 0x3E8 {
		return
	}
	g.loadFile(fnWoshi)
	g.setPalette(palBlack)
	g.decodeImage(0xA000)
	g.setPalette(palImage)
	g.sb(vRedraw, 1)
	g.poll()
	g.getKey()
}

// gameTime — L_1fa3: учёт игрового времени: первый вызов запоминает момент,
// второй прибавляет прошедшее время к статистике текущего игрока.
func (g *Game) gameTime() {
	if g.b(vTimerOn) != 1 {
		g.sb(vTimerOn, 1)
		ch, cl, dh, _ := g.dosTime()
		g.sb(vTimerH, ch)
		g.sb(vTimerM, cl)
		g.sb(vTimerS, dh)
		return
	}
	g.sb(vTimerOn, 0)
	g.sw(0x0252, 0)
	ch, cl, dh, _ := g.dosTime()
	if dh < g.b(vTimerS) {
		g.sb(0x0252, 1)
		dh += 0x3C - g.b(vTimerS)
	} else {
		dh -= g.b(vTimerS)
	}
	if cl < g.b(vTimerM) {
		g.sb(0x0253, 1)
		cl += 0x3C - g.b(vTimerM)
	} else {
		cl -= g.b(vTimerM)
		if g.b(0x0252) != 0 {
			cl--
		}
	}
	if ch < g.b(vTimerH) {
		al := byte(0x18)
		if g.b(0x0252) != 0 {
			al--
		}
		ch += al - g.b(vTimerH)
	} else {
		ch -= g.b(vTimerH)
		if g.b(0x0253) != 0 {
			ch--
		}
	}
	si := uint16(0x025F)
	if g.b(vDouble) != 1 && g.b(vCurPlayer) != 1 {
		si = 0x0268
	}
	dh += g.b(si + 2)
	if dh >= 0x3C {
		cl++
		dh -= 0x3C
	}
	g.sb(si+2, dh)
	cl += g.b(si + 1)
	if cl >= 0x3C {
		ch++
		cl -= 0x3C
	}
	g.sb(si+1, cl)
	g.sb(si, g.b(si)+ch)
	if g.b(vDouble) == 1 {
		g.sb(si+9, g.b(si))
		g.sb(si+10, g.b(si+1))
		g.sb(si+11, g.b(si+2))
	}
}
