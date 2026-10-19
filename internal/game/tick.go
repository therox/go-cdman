package game

// Переменные CD-Man (по игроку: байты [p], слова [2p]).
const (
	vMaze      = 0x008E // клетки лабиринта: индекс клетки 22 + 21·строка + столбец
	vDir       = 0x019F // текущее направление: 0 вправо, 8 влево, 10h вниз, 18h вверх
	vWant      = 0x01A1 // желаемое направление (0FFh — стоять)
	vSub       = 0x01A3 // подшаг 0..7 внутри клетки
	vCell      = 0x01A5 // клетка, к которой идёт CD-Man
	vFrom      = 0x01A9 // клетка, из которой он вышел
	vManAddr   = 0x01AD // адрес на странице 0
	vManSprite = 0x022F // последний выведенный кадр
	vTunnel    = 0x0068 // счётчик прохода через тоннель
	vInTunnel  = 0x006A
	vTunnelEnd = 0x006C // сторона тоннеля × 2
	vTunKind   = 0x0201 // вид тоннеля (как обрезается спрайт)
	vMoved     = 0x0257 // ход в клетку уже засчитан
	vLastCell  = 0x046B // последняя клетка, за которую дали очки на последнем экране
	vBack      = 0x021F // фон уходящей клетки
	vDeltas    = 0x1013 // смещения клеток по направлениям
	vCurMon    = 0x0221 // номер монстра
	vMonMode   = 0x007A // [m] «G» — охотится, «M» — синий
	vMonStep2  = 0x0227 // [m] каждый 15-й тик — лишний шаг
	vMonSkip   = 0x022B // [m] синий пропускает каждый 8-й тик
	vMonBlue   = 0x007E // [2m] время синего состояния
	vMonScore  = 0x0086 // [2m] время показа очков за монстра
	vMonDir    = 0x01B1 // [m] текущее направление
	vMonNext   = 0x01B5 // [m] выбранное направление (0FFh — стоять)
	vMonSub    = 0x01B9 // [m] подшаг
	vMonCell   = 0x01BD // [2m]
	vMonFrom   = 0x01C5 // [2m]
	vMonAddr   = 0x01CD // [2m]
	vMonWait   = 0x01D5 // [2m] ожидание в клетке монстров
	vFireCell  = 0x01DD // [2m] клетки огней
	vFireAddr  = 0x01E5 // [2m]
	vFireFrame = 0x005E // [2m] кадр огня (0FFh — съеден)
	vEaten     = 0x0066 // сколько монстров съедено подряд × 2
	vPrefs     = 0x100F // порядок предпочтения направлений монстра
	vTargetX   = 0x101B // [2p] столбец игрока (цель монстров)
	vTargetY   = 0x101F // [2p] строка
	vBonus     = 0x0000 // [4k] клетка и кадр всплывающих очков (0..3 монстры, 4 фрукт, 5 ключ)
	tileEmpty  = 0x6DA0 // пустая клетка (страница 1)
	tileDot    = 0x6DA4 // клетка с точкой
)

// tick — L_0aac: игровой тик: шаг CD-Man (двух в режиме Double), опрос
// клавиатуры, нота и задержка, шаги четырёх монстров с задержкой после каждого.
func (g *Game) tick() {
	if g.onTick != nil {
		g.onTick(g)
	}
	g.sw(vTurn, 0)
	g.sw(vTurn2, 0)
	skip2 := false
	if g.w(vLives) != 0 {
		if t := g.b(vTunnel); t < 0x14 && t > 8 {
			g.tunnel()
		} else {
			g.moveMan()
			g.levelDone()
			skip2 = g.w(vDots) == 0
		}
	}
	if !skip2 && g.w(vLives+2) != 0 && g.b(vDouble) == 1 {
		g.sw(vTurn, 1)
		g.sw(vTurn2, 2)
		if t := g.b(vTunnel + 1); t < 0x14 && t > 8 {
			g.tunnel()
		} else {
			g.moveMan()
			g.levelDone()
		}
	}
	if g.b(vDouble) == 1 {
		g.keysDouble()
	} else {
		g.keysSingle()
	}
	if g.b(vStop) == 1 {
		return
	}
	g.sound()
	g.target(0)
	if g.b(vDouble) == 1 {
		g.target(2)
	}
	for bx := uint16(0); bx < 4; bx++ {
		g.sw(vCurMon, bx)
		if g.b(vMonMode+bx) == 'M' {
			g.sb(vMonSkip+bx, g.b(vMonSkip+bx)+1)
			if g.b(vMonSkip+bx) == 8 {
				g.sb(vMonSkip+bx, 0)
			} else {
				g.moveMonster()
				g.thinkMonster()
			}
		} else {
			g.sb(vMonStep2+bx, g.b(vMonStep2+bx)+1)
			if g.b(vMonStep2+bx) == 0x0F {
				g.sb(vMonStep2+bx, 0)
				g.moveMonster()
				g.thinkMonster()
				g.delayTick()
			}
			g.moveMonster()
			g.thinkMonster()
		}
		g.delayTick()
	}
	g.spin(1750)
}

// keysSingle — CS:26D9: стрелки задают направление, Esc — меню, Ctrl+Q — выход.
func (g *Game) keysSingle() {
	ax := g.poll()
	g.arrowsOrKeys(ax, vWant)
}

// keysDouble — CS:2732: стрелки — второй CD-Man, W/A/D/X(Z) — первый.
func (g *Game) keysDouble() {
	ax := g.pollDouble()
	al := byte(ax)
	if al == 0 {
		g.arrowsOrKeys(ax, vWant+1)
		return
	}
	if al >= 0x61 {
		al -= 0x20
	}
	switch al {
	case 'W':
		g.sb(vWant, 0x18)
	case 'X', 'Z':
		g.sb(vWant, 0x10)
	case 'A':
		g.sb(vWant, 8)
	case 'D':
		g.sb(vWant, 0)
	default:
		g.otherKeys(ax)
	}
}

func (g *Game) arrowsOrKeys(ax, want uint16) {
	if byte(ax) != 0 {
		g.otherKeys(ax)
		return
	}
	switch byte(ax >> 8) {
	case scanUp:
		g.sb(want, 0x18)
	case scanDown:
		g.sb(want, 0x10)
	case scanLeft:
		g.sb(want, 8)
	case scanRight:
		g.sb(want, 0)
	}
}

func (g *Game) otherKeys(ax uint16) {
	if byte(ax) == asciiEsc {
		g.escMenu()
	}
	if ax == scanCtrlQ {
		g.exitNow()
	}
}

// sound — L_2615: очередная нота мелодии или «чавканье» по подшагу, если
// CD-Man идёт к точке; затем задержка L_2682.
func (g *Game) sound() {
	si := g.w(vSndTune)
	var ax uint16
	playing := false
	if si != tuneNone {
		if g.w(vSndPos) != g.w(si) {
			si += 2 + g.w(vSndPos)
			g.addw(vSndPos, 2)
			ax = g.w(si)
			playing = true
		} else {
			g.sw(vSndPos, 0)
			si = tuneNone
			g.sw(vSndTune, si)
		}
	}
	if !playing {
		ax = 0
		if g.b(vMaze+g.w(vCell))&8 != 0 {
			ax = g.w(si + uint16(g.b(vSub))<<1)
		}
		if g.b(vDouble) == 1 {
			si = tuneNone
			if g.b(vMaze+g.w(vCell+2))&8 != 0 {
				ax += g.w(si + uint16(g.b(vSub+1))<<1)
			}
		}
	}
	g.speaker(ax)
	g.delayTick()
}

// speaker — L_26b1: делитель канала 2 таймера (0 или звук выключен — тишина).
func (g *Game) speaker(ax uint16) {
	if g.b(vSoundOn) == 0 || ax == 0 {
		ax = 0
	}
	g.snd.Sync(g.clk.Now())
	g.snd.Divisor(g.clk.Now(), ax)
}

// target — L_27bd: строка и столбец игрока si/2 — цель монстров. В режиме
// Double, когда один погиб, оба замера берутся у живого.
func (g *Game) target(si uint16) {
	bx := si
	if g.b(vAlive) != 2 {
		bx = uint16(1-g.b(vAlive)) << 1
	}
	ax := g.w(vCell + bx)
	row := uint16(0)
	for ax > 0x28 {
		ax -= 0x15
		row++
	}
	g.sw(vTargetX+si, ax-0x16)
	g.sw(vTargetY+si, row)
}

// cellRowCol — номер клетки в столбец и строку (как в L_27bd и L_2874).
func cellRowCol(c uint16) (col, row uint16) {
	for c > 0x28 {
		c -= 0x15
		row++
	}
	return c - 0x16, row
}

// tunnel — L_3338: CD-Man в клетке входа в тоннель и идёт к нему —
// начинается проход (27 шагов). Во время прохода счётчик уменьшается.
func (g *Game) tunnel() {
	bx := g.w(vTurn)
	si := g.w(vTurn2)
	if g.b(vTunnel+bx) != 0 {
		g.sb(vTunnel+bx, g.b(vTunnel+bx)-1)
		return
	}
	g.sb(vInTunnel+bx, 0)
	bp := g.w(vCell + si)
	enter := func(dirVar, kind uint16, side uint16) bool {
		cl := g.b(dirVar)
		if cl != g.b(vDir+bx) {
			if cl != g.b(vWant+bx) {
				return false
			}
			g.sb(vDir+bx, cl)
		}
		if side == 2 && g.w(vScreen) == 8 && g.b(vWant+bx) == 0x10 {
			return false
		}
		g.sw(vTunnelEnd+si, side)
		g.sb(vTunKind+bx, g.b(kind))
		return true
	}
	ok := false
	switch bp {
	case g.w(0x01ED):
		ok = enter(0x01FD, 0x0203, 0)
	case g.w(0x01EF):
		ok = enter(0x01FE, 0x0204, 2)
	}
	if ok {
		g.sb(vTunnel+bx, 0x1B)
		g.sb(vInTunnel+bx, 1)
	}
}

// tunnelExit — CS:33CD и CS:33FF: перенести CD-Man к другому выходу тоннеля.
func (g *Game) tunnelExit(side uint16) {
	bx := g.w(vTurn)
	si := g.w(vTurn2)
	if side == 0 {
		cl := g.b(0x0200)
		g.sb(vWant+bx, cl)
		g.sb(vDir+bx, cl)
		g.sw(vManAddr+si, g.w(0x01F7))
		g.sw(vCell+si, g.w(0x01F3))
		g.sb(vTunKind+bx, g.b(0x0204))
		return
	}
	cl := g.b(0x01FF)
	g.sb(vWant+bx, cl)
	g.sb(vDir+bx, cl)
	g.sw(vManAddr+si, g.w(0x01F5))
	g.sw(vCell+si, g.w(0x01F1))
	g.sb(vTunKind+bx, g.b(0x0203))
}

// dirBit — бит клетки, разрешающий направление dir (10h, 20h, 40h, 80h).
func dirBit(dir byte) byte {
	cl := (dir>>3 | dir<<5) & 7
	return 0x10 << cl
}

// blockedByOther — в режиме Double клетка в направлении dir от bp занята
// другим CD-Man.
func (g *Game) blockedByOther(si uint16, dir byte, bp uint16) bool {
	other := ((si + 1) & 1) << 1
	di := g.w(vDeltas+uint16(dir)>>2) + bp
	return di == g.w(vCell+other)
}

// moveMan — L_3431: шаг CD-Man текущего игрока и его вывод.
func (g *Game) moveMan() {
	si := g.w(vTurn)
	if g.b(vWant+si) == 0xFF {
		g.drawStanding()
		return
	}
	bx := g.w(vTurn2)
	g.leavingTile(bx)
	bp := g.w(vCell + bx)
	bl := (g.b(vSub+si) + 1) & 7
	g.sb(vSub+si, bl)
	var dh byte
	if bl != 0 {
		g.sb(vMoved+si, 0)
		dh = g.b(vDir + si)
		dl := g.b(vWant + si)
		if dh == dl {
			g.manStep(dh)
			return
		}
		if g.b(vDouble) == 1 && g.blockedByOther(si, dl, bp) {
			g.manStep(dh)
			return
		}
		if (dl^dh)&0x10 != 0 {
			g.manStep(dh)
			return
		}
		// разворот на месте
		if g.b(vInTunnel+si) != 0 {
			g.sb(vTunnel+si, 0x1A-g.b(vTunnel+si))
			dh = g.b(vWant + si)
			g.sb(vDir+si, dh)
			g.sb(vSub+si, -g.b(vSub+si)&7)
			s2 := si << 1
			a, b := g.w(vCell+s2), g.w(vFrom+s2)
			g.sw(vCell+s2, b)
			g.sw(vFrom+s2, a)
			g.sw(vTunnelEnd+s2, (g.w(vTunnelEnd+s2)+2)&2)
			g.manStep(dh)
			return
		}
		dh = g.b(vWant + si)
		g.sb(vDir+si, dh)
		g.sb(vSub+si, -g.b(vSub+si)&7)
		if bl < 6 {
			g.manAdvance(bp, dh)
			return
		}
	}
	// L_3529: CD-Man в клетке bp
	if g.b(vMoved+si) != 1 {
		g.sb(vMoved+si, 1)
		st := uint16(vStats1)
		if g.b(vCurPlayer) == 2 || g.w(vTurn) == 1 {
			st = vStats2
		}
		g.incw(st)
	}
	if g.b(vMaze+bp)&8 != 0 {
		s2 := g.w(vTurn2)
		if g.b(vLastPass) == 1 {
			if bp != g.w(vLastCell+s2) {
				g.sw(vLastCell+s2, bp)
				g.incw(vScore + s2)
				g.drawScore()
			}
		} else {
			g.incw(vScore + s2)
			g.drawScore()
			g.decw(vDots)
			g.sb(vMaze+bp, g.b(vMaze+bp)&0xF7)
		}
		if bl != 0 {
			g.manAdvance(bp, g.b(vDir+si))
			return
		}
	}
	g.tunnel()
	si = g.w(vTurn)
	dh = g.b(vDir + si)
	if g.b(vTunnel+si) != 0 {
		g.manAdvance(bp, dh)
		return
	}
	// поворот, если можно. В BX оригинала здесь младший байт — подшаг; другой
	// CD-Man попадает в BX, только если желаемое направление открыто.
	bxv := uint16(bl)
	dh = g.b(vWant + si)
	if g.b(vMaze+bp)&dirBit(dh) != 0 {
		if g.b(vDouble) != 1 {
			g.sb(vDir+si, dh)
			g.manAdvance(bp, dh)
			return
		}
		bxv = ((si + 1) & 1) << 1
		if g.w(vDeltas+uint16(dh)>>2)+bp != g.w(vCell+bxv) {
			g.sb(vDir+si, dh)
			g.manAdvance(bp, dh)
			return
		}
	}
	// L_35e6: дальше в прежнем направлении, если там не стена
	dh = g.b(vDir + si)
	if g.b(vMaze+bp)&dirBit(dh) != 0 {
		if g.b(vDouble) != 1 || g.w(vDeltas+uint16(dh)>>2)+bp != g.w(vCell+bxv) {
			g.manAdvance(bp, dh)
			return
		}
	}
	// стена впереди: стоим
	g.sb(vSub+si, 7)
	g.drawStanding()
}

// manAdvance — L_3620: перейти к следующей клетке в направлении dh.
func (g *Game) manAdvance(bp uint16, dh byte) {
	bx := g.w(vTurn2)
	g.sw(vFrom+bx, bp)
	switch dh {
	case 0:
		g.incw(vCell + bx)
	case 8:
		g.decw(vCell + bx)
	case 0x10:
		g.addw(vCell+bx, 0x15)
	default:
		g.sw(vCell+bx, g.w(vCell+bx)-0x15)
	}
	g.leavingTile(bx)
	g.manStep(dh)
}

// manStep — L_34a2: сдвиг спрайта на шаг в направлении dh со стиранием следа.
func (g *Game) manStep(dh byte) {
	bx := g.w(vTurn2)
	si := g.w(vTurn)
	p := bx >> 1
	di := g.w(vManAddr + bx)
	inTun := g.b(vInTunnel+si) == 1
	t := g.b(vTunnel + si)
	odd := g.b(vSub+p)&1 != 0
	switch dh {
	case 0x10: // L_3691 вниз
		if t <= 8 && inTun {
			di += 0xF0
		} else {
			g.eraseAbove(di, g.b(vSub+p))
			di += 0xF0
		}
		g.sw(vManAddr+bx, di)
		g.drawMan(di)
	case 0: // L_36b5 вправо
		if t <= 8 && inTun {
			if odd {
				di++
				g.sw(vManAddr+bx, di)
				g.drawMan(di)
				return
			}
			g.drawMan5(di)
			return
		}
		if !odd {
			g.drawMan5(di)
			return
		}
		g.eraseLeft(di, g.b(vSub+p))
		di++
		g.sw(vManAddr+bx, di)
		g.drawMan(di)
	case 8: // L_36f4 влево
		if t <= 8 && inTun {
			if odd {
				g.drawMan(di)
				return
			}
		} else if odd {
			g.eraseRight(di, g.b(vSub+p))
			g.drawMan(di)
			return
		}
		di--
		g.sw(vManAddr+bx, di)
		g.drawMan5(di)
	default: // L_3665 вверх
		if t <= 0x0A && inTun {
			di -= 0xF0
		} else {
			di += 0x690
			g.eraseBelow(di, g.b(vSub+p))
			di -= 0x870 - 0xF0
		}
		g.sw(vManAddr+bx, di)
		g.drawMan(di)
	}
}

// drawStanding — L_372c: вывести последний кадр на месте.
func (g *Game) drawStanding() {
	bx := g.w(vTurn2)
	si := g.w(vManSprite + bx)
	di := g.w(vManAddr + bx)
	p := bx >> 1
	if g.b(vSub+p)&1 == 0 && g.b(vDir+p)&0x10 == 0 {
		g.copyRows(si, di, 5, 0x18)
	} else {
		g.copyRows(si, di, 4, 0x18)
	}
}

// leavingTile — L_3753: фон клетки, которую покидает CD-Man (g_021f).
func (g *Game) leavingTile(bx uint16) {
	g.sw(vBack, tileEmpty)
	bp := g.w(vFrom + bx)
	if g.b(vMaze+bp)&4 != 0 {
		g.bonusTile(bp)
		return
	}
	if g.b(vMaze+bp)&8 != 0 {
		g.sw(vBack, tileDot)
	}
}

// bonusTile — L_3cbd: кадр всплывающих очков или бонуса в клетке bp.
func (g *Game) bonusTile(bp uint16) {
	for bx := uint16(0); bx != 0x18; bx += 4 {
		if bp == g.w(vBonus+bx) {
			g.sw(vBack, g.w(vBonus+bx+2))
			return
		}
	}
}

// copyRows копирует w байт × h строк страницы через защёлки.
func (g *Game) copyRows(si, di uint16, w, h int) {
	g.v.CopyBlock(si, di, w, h)
	g.spin(float64(h * (w*2 + 3)))
}

// eraseBelow — L_39fe: при шаге вверх вернуть фон трёх строк под спрайтом.
func (g *Game) eraseBelow(di uint16, sub byte) {
	si := g.w(vBack) + 0x780
	for n := int(sub) + 1; n > 0; n-- {
		si -= 0xF0
	}
	g.copyRows(si, di, 4, 3)
}

// eraseAbove — L_3a42: при шаге вниз вернуть фон трёх строк над спрайтом.
func (g *Game) eraseAbove(di uint16, sub byte) {
	si := g.w(vBack) - 0xF0
	for n := int(sub) + 1; n > 0; n-- {
		si += 0xF0
	}
	g.copyRows(si, di, 4, 3)
}

// eraseLeft — L_3a86: при шаге вправо вернуть фон левого байта.
func (g *Game) eraseLeft(di uint16, sub byte) {
	si := g.w(vBack) + uint16(sub>>1)
	g.copyRows(si, di, 1, 0x18)
}

// eraseRight — L_3aaa: при шаге влево вернуть фон правого байта.
func (g *Game) eraseRight(di uint16, sub byte) {
	si := g.w(vBack) + 3 - uint16(sub>>1)
	g.copyRows(si, di+4, 1, 0x18)
}

// drawMan — L_3778: кадр CD-Man шириной 4 байта (с точкой на фоне, если она
// есть в клетке назначения).
func (g *Game) drawMan(di uint16) {
	p := g.w(vTurn)
	bx := uint16(g.b(vDir+p)) * 2
	cell := g.w(vCell + p*2)
	odd := g.b(vSub+p)&1 != 0
	var si uint16
	if g.b(vMaze+cell)&8 != 0 {
		si = 0x6D60
		if !odd {
			si = 0x74C0
		}
	} else {
		si = 0xDAC0
		if !odd {
			si = 0xE220
		}
	}
	dx := uint16(g.b(vSub+p) & 0xFE)
	si += bx + dx + dx
	g.sw(vManSprite+p*2, si)
	if g.b(vTunnel+p) != 0 {
		g.drawManTunnel(si, di, 4)
		return
	}
	g.copyRows(si, di, 4, 0x18)
}

// drawMan5 — L_38f3: кадр шириной 5 байт (CD-Man сдвинут на полбайта).
func (g *Game) drawMan5(di uint16) {
	p := g.w(vTurn)
	d := uint16(g.b(vDir + p))
	bx := d + d + d>>1
	dx := uint16(g.b(vSub+p) & 0xFE)
	bx += dx + dx + dx>>1
	cell := g.w(vCell + p*2)
	si := uint16(0xE260)
	if g.b(vMaze+cell)&8 != 0 {
		si = 0xC760
	}
	si += bx
	g.sw(vManSprite+p*2, si)
	if g.b(vTunnel+p) != 0 {
		g.drawManTunnel(si, di, 5)
		return
	}
	g.copyRows(si, di, 5, 0x18)
}

// drawManTunnel — L_3803 / L_3971: CD-Man в тоннеле — спрайт обрезается по
// краю тоннеля; на подшаге 7 он переносится к другому выходу.
func (g *Game) drawManTunnel(si, di uint16, width int) {
	p := g.w(vTurn)
	g.sb(vTunnel+p, g.b(vTunnel+p)-1)
	if g.b(vSub+p) == 7 {
		g.tunnelExit(g.w(vTunnelEnd + p*2))
		return
	}
	t := g.b(vTunnel + p)
	sub := g.b(vSub + p)
	kind := g.b(vTunKind + p)
	rows := 0x18
	bx := width
	if width == 4 {
		switch {
		case kind == 0:
			cl := int(sub) + 1
			if t <= 0x0A {
				cl = 8 - cl
			}
			for ; cl > 0; cl-- {
				rows -= 3
				si += 0xF0
				di += 0xF0
			}
		case kind == 1:
			dl := int(sub)*3 + 3
			if t <= 0x0A {
				dl = 0x18 - dl
			}
			rows -= dl
		case kind == 3:
			bl := int(sub>>1) + 1
			if t >= 0x0A {
				bl = 4 - bl
			}
			bx = bl
		default: // 2
			bl := int(sub>>1) + 1
			dx := uint16(bl)
			if t < 0x0A {
				si += uint16(4 - bl)
				di += uint16(4 - bl)
			} else {
				bl = 4 - bl
				si += dx
				di += dx
			}
			bx = bl
		}
	} else {
		bl := int(sub>>1) + 1
		if kind == 3 {
			if t > 8 {
				bl = 5 - bl
			}
		} else {
			dx := uint16(bl)
			if t < 0x0A {
				si += uint16(5 - bl)
				di += uint16(5 - bl)
			} else {
				bl = 5 - bl
				si += dx
				di += dx
			}
		}
		bx = bl
	}
	g.copyRows(si, di, bx&0xFF, rows&0xFF)
}

// levelDone — L_328c: все точки съедены — CD-Man мигает под мелодию.
func (g *Game) levelDone() {
	if g.w(vDots) != 0 || g.w(vLives+g.w(vTurn2)) == 0 {
		return
	}
	bx := g.w(vTurn)
	if g.b(vInTunnel+bx) != 0 {
		di := g.w(vTunnelEnd + bx*2)
		if g.b(vTunnel+bx) <= 0x0A {
			di = (di + 2) & 2
		}
		di += 4
		g.sw(vManAddr+g.w(vTurn2), g.w(0x01F5+di))
	} else if g.b(vSub+bx)&1 == 0 {
		g.sb(vWant+bx, g.b(vDir+bx))
		g.moveMan()
	}
	g.sw(vSndPos, 0)
	g.sw(vSndTune, 0x0597)
	si := uint16(0xB860)
	for i := 0; i < 0x10; i++ {
		di := g.w(vManAddr + g.w(vTurn2))
		g.copyRows(si, di, 4, 0x18)
		g.sound()
		if g.b(vGate) != 'C' {
			g.sb(vGateAnim, 'C')
			g.gate()
		}
		si += 4
		g.sw(vDelayMul, 0x64)
		g.waitN()
	}
	g.sw(vDelayMul, 0x5DC)
	g.waitN()
}
