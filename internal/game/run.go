package game

import "fmt"

// Переменные главного цикла.
const (
	vQuit       = 0x0077 // выход из программы
	vNewGame    = 0x0075 // сразу начать новую игру
	vStop       = 0x0074 // прекратить текущую игру
	vInGame     = 0x0615 // игра начата (меню вызвано из игры)
	vPlayers    = 0x0616 // игроков: 1 или 2
	vPlayersSet = 0x0617 // выбрано в меню
	vDouble     = 0x0618 // режим Double: два CD-Man одновременно
	vDoubleSet  = 0x0619
	vComputer   = 0x061B // «One player vs Computer» (в меню не выбирается)
	vAlive      = 0x061A // в режиме Double: 0/1 — жив только этот игрок, 2 — оба
	vAfterGame  = 0x2077 // после игры: ввод имени в таблицу рекордов
	vEnding     = 0x2078 // палитра не восстанавливается после меню
	vSessH      = 0x139E // время начала сеанса (часы, минуты, секунды)
	vSessM      = 0x139F
	vSessS      = 0x13A0
	vSndPos     = 0x055B // позиция в мелодии
	vSndTune    = 0x055D // текущая мелодия
	tuneNone    = 0x055F
	vSoundOn    = 0x055A
	vCalibStart = 0x24F7 // момент начала замера (DH секунды, DL сотые)
)

// Run — программа целиком: заставка, меню, игры. Возвращается при выходе
// из программы (пункт «Quit», Ctrl+Q или отсутствие файла).
func (g *Game) Run() (err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(exitDOS); ok {
				return
			}
			if _, ok := r.(quitNow); ok {
				g.speakerOff()
				g.quit()
				return
			}
			panic(r)
		}
	}()
	g.entry()
	return nil
}

// entry — точка входа CS:0000.
func (g *Game) entry() {
	// распределение памяти DOS (значения как при загрузке с сегмента 1010h)
	seg := uint16(0x1386 + 0x1000)
	g.sw(segInit, seg)
	seg += initSize>>4 + 1
	for _, a := range []uint16{segP1, segP2, segP3, segP4, segBuf} {
		g.sw(a, seg)
		seg += playerSize>>4 + 1
	}
	g.reseed()
	g.sb(vSeedUses, 0)
	g.loadCharTab()
	g.graphMode(false)
	g.main()
	g.v.SetMode(3)
}

// Метки главного цикла.
const (
	lMenu = iota
	lGame
	lAfter
)

// main — L_0098.
func (g *Game) main() {
	ch, cl, dh, _ := g.dosTime()
	g.sb(vSessH, ch)
	g.sb(vSessM, cl)
	g.sb(vSessS, dh)
	g.saveState(segInit)
	g.initCellAddrs()
	g.loadHiScores()
	g.calibrate()
	g.title()
	g.setTextFonts()

	label := lMenu
	for {
		switch label {
		case lMenu: // L_00c0
			if g.catchMain(g.menuPart) {
				label = lAfter
				continue
			}
			if g.b(vQuit) == 1 {
				g.quit()
				return
			}
			label = lGame
		case lGame: // L_00ee
			g.catchMain(g.gamePart)
			label = lAfter
		case lAfter: // L_0127
			g.speakerOff()
			g.sw(vSndPos, 0)
			g.sw(vSndTune, tuneNone)
			if g.b(vQuit) == 1 {
				g.quit()
				return
			}
			if g.b(vNewGame) == 1 {
				label = lGame
				continue
			}
			g.sb(vInGame, 0)
			label = lMenu
		}
	}
}

// catchMain выполняет f; true — если f завершилась переходом на L_0127.
func (g *Game) catchMain(f func()) (jumped bool) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(toMain); ok {
				jumped = true
				return
			}
			panic(r)
		}
	}()
	f()
	return false
}

// menuPart — L_00c0..L_00ec: текстовый режим, таблица рекордов после игры, меню.
func (g *Game) menuPart() {
	g.textMode(false)
	g.setTextFonts()
	if g.b(vAfterGame) == 1 {
		g.afterGame()
	}
	g.mainMenu()
}

// gamePart — L_00ee..L_0124: начальное состояние, графический режим и игра.
func (g *Game) gamePart() {
	g.restoreState(segInit)
	g.graphMode(false)
	g.sb(vPlayers, g.b(vPlayersSet))
	g.sb(vDouble, g.b(vDoubleSet))
	g.sb(vInGame, 1)
	switch {
	case g.b(vDouble) == 1:
		g.playDouble()
	case g.b(vPlayers) == 1:
		g.playOne()
	default:
		g.playTwo()
	}
}

// quit — L_0155: учесть время сеанса и сохранить таблицу рекордов.
func (g *Game) quit() {
	g.addSessionTime()
	g.saveHiScores()
}

// speakerOff — «in al, 61h; and al, 0FCh; out 61h, al».
func (g *Game) speakerOff() {
	g.snd.Sync(g.clk.Now())
	g.snd.Divisor(g.clk.Now(), 0)
}

// calibrate — L_015c: замер скорости машины и выбор задержки.
func (g *Game) calibrate() {
	g.printDOS(0x24F9)
	_, _, dh, dl := g.dosTime()
	g.sw(vCalibStart, uint16(dh)<<8|uint16(dl))
	// 16 × 40000 чтений видеопамяти: «mov al, es:[si]; loop»
	for i := 0; i < 16; i++ {
		g.spin(40000*2 + 4)
	}
	bx := g.elapsed()
	var dx uint16
	switch {
	case bx <= 0x6E:
		dx = 0x990 - bx<<4
	case bx <= 0xB6:
		dx = 0x468 - bx<<2
	case bx <= 0xF6:
		dx = 0x2FD - bx<<1
	case bx <= 0x14C:
		dx = 0x207 - bx
	case bx <= 0x1C8:
		dx = 0x161 - bx>>1
	default:
		dx = 0xEF - bx>>2
	}
	g.sw(vDelay, dx)
	g.sw(vDelayStep, max(dx>>6, 1))
	for _, lim := range []uint16{0x64, 0xC8, 0x12C} {
		if g.w(vDelay) < lim {
			break
		}
		g.sb(0x049D, g.b(0x049D)>>1)
	}
	g.sw(vDelaySaved, g.w(vDelay))
}

// elapsed — L_022d: сколько сотых секунды прошло с момента g_24f7
// (с той же погрешностью, что в оригинале).
func (g *Game) elapsed() uint16 {
	_, _, dh, dl := g.dosTime()
	ax := g.w(vCalibStart)
	ah, al := byte(ax>>8), byte(ax)
	bx := uint16(dl)
	if ah == dh {
		return bx - uint16(al)
	}
	if int8(ah) > int8(dh) {
		dh += 60 - ah
		ah = 0
	}
	for {
		dh--
		if ah == dh {
			break
		}
		bx += 100
	}
	return bx + (99 - uint16(al))
}

// initCellAddrs — L_0262: адреса клеток лабиринта на странице 0.
func (g *Game) initCellAddrs() {
	ax := uint16(0x1222)
	di := uint16(0x2C)
	for r := 0; r < 11; r++ {
		for c := 0; c < 19; c++ {
			g.sw(vCellAddr+di, ax)
			ax += 4
			di += 2
		}
		ax += 0x734
		di += 4
	}
}

// vCellAddr — g_061c: адрес клетки на странице 0 (индекс клетки × 2).
const vCellAddr = 0x061C

// loadHiScores — L_23ce: прочитать HIGHSC.CDM; если его нет — проставить
// в таблицах текущие дату и время.
func (g *Game) loadHiScores() {
	if h, ok := g.open(fnHiScore); ok {
		g.sw(vHandle, h)
		g.readFile(h, g.ds[hiScoreOff:hiScoreOff+hiScoreLen])
		g.closeFile(h)
		return
	}
	year, mon, day := g.dosDate()
	cx := year - 0x708
	for {
		cx -= 0x64
		if cx < 0x64 {
			break
		}
	}
	g.sb(0x0296, byte(cx))
	date := func(di uint16) uint16 {
		di = g.digits2(di, g.b(0x0296))
		di = g.digits2(di, mon)
		return g.digits2(di, day)
	}
	di := uint16(0x199B + 0x15A)
	for i := 0; i < 10; i++ {
		di = date(di) + 0x40
	}
	di += 0x4A
	di = date(di) + 0x41
	di = date(di) + 0x41
	date(di)
	hh, mm, _, _ := g.dosTime()
	clock := func(di uint16) uint16 {
		di = g.digits2(di, hh)
		return g.digits2(di, mm)
	}
	di = 0x199B + 0x165
	for i := 0; i < 10; i++ {
		di = clock(di) + 0x43
	}
	di += 0x4A
	di = clock(di) + 0x44
	di = clock(di) + 0x44
	clock(di)
}

// digits2 — L_23a0: записать число 0..99 двумя цифрами в DS:di, пропустить разделитель.
func (g *Game) digits2(di uint16, al byte) uint16 {
	ah := byte(0)
	for al >= 10 {
		al -= 10
		ah++
	}
	g.sb(di, ah+'0')
	g.sb(di+1, al+'0')
	return di + 3
}

// saveHiScores — L_245d: записать HIGHSC.CDM.
func (g *Game) saveHiScores() {
	data := append([]byte(nil), g.ds[hiScoreOff:hiScoreOff+hiScoreLen]...)
	if err := g.fs.Write("HIGHSC.CDM", data); err != nil {
		fmt.Println("cdman:", err)
	}
}

// addSessionTime — L_226d: прибавить длительность сеанса к общему времени игры
// (недели, дни, часы, минуты, секунды в DS:196Ch..1975h).
func (g *Game) addSessionTime() {
	ch0, cl0, dh0, _ := g.dosTime()
	ch, cl, dh := ch0, cl0, dh0
	g.sw(0x13A1, 0)
	if dh < g.b(vSessS) {
		g.sb(0x13A1, 1)
		dh += 0x3C - g.b(vSessS)
	} else {
		dh -= g.b(vSessS)
	}
	if cl < g.b(vSessM) {
		g.sb(0x13A2, 1)
		cl += 0x3C - g.b(vSessM)
	} else {
		cl -= g.b(vSessM)
		if g.b(0x13A1) != 0 {
			cl--
		}
	}
	if ch < g.b(vSessH) {
		al := byte(0x18)
		if g.b(0x13A1) != 0 {
			al--
		}
		ch += al - g.b(vSessH)
	} else {
		ch -= g.b(vSessH)
		if g.b(0x13A2) != 0 {
			ch--
		}
	}
	g.sb(vSessH, ch)
	g.sb(vSessM, cl)
	g.sb(vSessS, dh)
	g.addTime(0x196C, vSessH)
	g.sb(vSessH, ch0)
	g.sb(vSessM, cl0)
	g.sb(vSessS, dh0)
}

// addTime прибавляет часы, минуты, секунды из DS:si к счётчику по адресу t
// (слова: недели, дни, часы, минуты, секунды).
func (g *Game) addTime(t, si uint16) {
	ax, bx, cx, dx := g.w(t+2), g.w(t+4), g.w(t+6), g.w(t+8)
	dx = dx&0xFF00 | uint16(byte(dx)+g.b(si+2))
	if dx >= 0x3C {
		cx = cx&0xFF00 | uint16(byte(cx)+1)
		dx -= 0x3C
	}
	g.sw(t+8, dx)
	cx = cx&0xFF00 | uint16(byte(cx)+g.b(si+1))
	if cx >= 0x3C {
		bx = bx&0xFF00 | uint16(byte(bx)+1)
		cx -= 0x3C
	}
	g.sw(t+6, cx)
	bx = bx&0xFF00 | uint16(byte(bx)+g.b(si))
	for bx >= 0x18 {
		bx -= 0x18
		ax++
	}
	g.sw(t+4, bx)
	if ax > 7 {
		ax -= 7
		g.incw(t)
	}
	g.sw(t+2, ax)
}
