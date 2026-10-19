package game

// Бонусы и особенности экранов.
const (
	vFruit     = 0x0037 // фрукт: 0 нет, 1 лежит, 2 показаны очки за него
	vFruitTime = 0x0038
	vFruitCell = 0x0010
	vFruitAddr = 0x003A
	vFruitSpr  = 0x0012
	vFruitType = 0x003C
	vKey       = 0x0040 // ключ (экран 1) лежит
	vKeyTime   = 0x0041
	vKeyCell   = 0x0014
	vKeyAddr   = 0x0043
	vKeySpr    = 0x0016
	vKeyScore  = 0x0045 // время показа очков за второй ключ
	vHasKey    = 0x003E // [p] у игрока есть ключ
	vGate      = 0x0048 // ворота: «O» открыты, «C» закрыты
	vGateAnim  = 0x0049 // «O» открываются, «C» закрываются, «N» стоят
	vGateFrame = 0x004A
	vGateTimer = 0x004C
	vGateBusy  = 0x0047 // в воротах кто-то есть
	vDoor      = 0x005C // дверь клетки монстров: 55h — открывается
	vDoorFrame = 0x005A
	vFireWait  = 0x0035 // последний экран: задержка возврата огня
	vFireStep  = 0x0033
	vMazes     = 0x048D // тексты лабиринтов
	vAnimTable = 0x049E // анимации экранов
)

// animate — L_2a11: фрукт, дверь клетки монстров, таймеры и огни, анимации экрана.
func (g *Game) animate() {
	g.fruit()
	g.doorAnim()
	g.timers()
	switch g.w(vAnimTable + g.w(vScreen)) {
	case 0x2A22: // экран 1
		g.key()
		g.sw(vTurn, 0)
		g.sw(vTurn2, 0)
		g.gate()
		if g.b(vDouble) == 1 {
			g.sw(vTurn, 1)
			g.sw(vTurn2, 2)
			g.gate()
		}
		g.anim1a()
		g.anim1b()
	case 0x2A51: // экран 2
		g.anim2a()
		g.anim2b()
		g.anim2c()
	case 0x2A5D: // экран 5
		g.anim5()
		g.lastFires()
	}
}

// randomCell — случайная открытая клетка без огня (L_2df8, L_3081).
func (g *Game) randomCell() uint16 {
	for {
		bx := g.random()>>8 + 0x0B
		if g.b(vMaze+bx)&1 == 0 || g.b(vMaze+bx)&2 != 0 {
			continue
		}
		return bx
	}
}

// cellTile — фон клетки: точка или пусто (и снять признак бонуса).
func (g *Game) clearBonus(cell uint16) uint16 {
	g.sb(vMaze+cell, g.b(vMaze+cell)&0xFB)
	if g.b(vMaze+cell)&8 != 0 {
		return tileDot
	}
	return tileEmpty
}

// fruit — L_2ccf: фрукт появляется в случайной клетке, лежит, его можно
// съесть (очки случайного уровня), затем исчезает.
func (g *Game) fruit() {
	if g.b(vFruit) == 0 {
		g.fruitWait()
		return
	}
	if g.decw(vFruitTime) == 0 {
		// L_2d89
		g.sb(vFruit, 0)
		g.sw(vFruitTime, g.random()>>8&0x7F+0x150)
		si := g.clearBonus(g.w(vFruitCell))
		g.copyRows(si, g.w(vFruitAddr), 4, 0x18)
		g.sw(vFruitCell, 0)
		g.fruitWait()
		return
	}
	bp := g.w(vFruitCell)
	di := uint16(0)
	g.sw(vTurn, 0)
	g.sw(vTurn2, 0)
	if bp != g.w(vCell) {
		if g.b(vDouble) != 1 {
			return
		}
		di = 2
		g.sw(vTurn, 1)
		g.sw(vTurn2, 2)
		if bp != g.w(vCell+2) {
			return
		}
	}
	if g.b(vFruit) != 1 {
		return
	}
	g.sb(vFruit, 2)
	g.sw(vFruitTime, 0x32)
	si := g.random() >> 8 & 6
	ax := g.w(0x0520 + si)
	bx := g.w(0x0528 + si)
	t := g.w(vFruitType)
	ax += g.w(0x0500 + t)
	bx += g.w(0x0510 + t)
	g.sw(vFruitSpr, ax)
	g.addw(vScore+di, bx)
	g.drawScore()
	g.sw(vBack, g.w(vFruitSpr))
	g.copyRows(g.w(vFruitSpr), g.w(vFruitAddr), 4, 0x18)
	g.sw(vSndPos, 0)
	g.sw(vSndTune, 0x0583)
	st := uint16(vStats1 + 4)
	if g.b(vCurPlayer) == 2 || g.w(vTurn) == 1 {
		st = vStats2 + 4
	}
	g.incw(st)
}

// fruitWait — L_2dbf: ожидание появления фрукта.
func (g *Game) fruitWait() {
	if g.decw(vFruitTime) != 0 {
		return
	}
	g.sb(vFruit, 1)
	g.sw(vFruitTime, g.random()>>8&0x3F+0xA8)
	g.sw(vFruitType, g.random()>>8&0x0E)
	ax := g.w(0x04F0 + g.w(vFruitType))
	ax += g.w(0x0520 + (g.w(vRound)-1)<<1)
	g.sw(vFruitSpr, ax)
	bx := g.randomCell()
	g.sw(vFruitCell, bx)
	g.sb(vMaze+bx, g.b(vMaze+bx)|4)
	di := g.w(vCellAddr + bx*2)
	g.sw(vFruitAddr, di)
	g.copyRows(g.w(vFruitSpr), di, 4, 0x18)
}

// doorAnim — L_2c3d: дверь клетки монстров открывается перед выходом монстра.
func (g *Game) doorAnim() {
	g.sw(vDoor, 0x44)
	for bx := uint16(0); bx < 8; bx += 2 {
		v := g.w(vMonWait + bx)
		switch {
		case v == 0:
		case v < 8:
			if g.w(vDoor) != 0x55 {
				g.sw(vDoor, 0x44)
			}
		case v > 0x2E:
		case v&1 == 0:
			return
		default:
			g.sw(vDoor, 0x55)
		}
	}
	bx := g.w(vScreen)
	if g.w(vDoor) == 0x55 {
		if g.w(vDoorFrame) == 0x20 {
			return
		}
		g.addw(vDoorFrame, 4)
		g.copyRows(g.w(0x004E)+g.w(vDoorFrame), g.w(0x0050+bx), 4, 0x18)
		return
	}
	if g.w(vDoorFrame) == 0xFFFC {
		return
	}
	g.copyRows(g.w(0x004E)+g.w(vDoorFrame), g.w(0x0050+bx), 4, 0x18)
	g.sw(vDoorFrame, g.w(vDoorFrame)-4)
}

// timers — L_2e2d: время синих монстров, всплывающие очки за монстров,
// огни: анимация или поедание (все монстры становятся синими).
func (g *Game) timers() {
	for bx := uint16(0); bx < 8; bx += 2 {
		if g.w(vMonBlue+bx) != 0 {
			g.decw(vMonBlue + bx)
			switch g.w(vMonBlue + bx) {
			case 0x32:
				g.monsterSprites(sprEnding, bx)
			case 0:
				g.sb(vMonMode+bx>>1, 'G')
				g.monsterSprites(sprNormal, bx)
			}
		}
		if g.w(vMonScore+bx) != 0 {
			if g.decw(vMonScore+bx) == 0 {
				di := g.w(vBonus + bx*2)
				g.sw(vBonus+bx*2, 0)
				g.sw(vBonus+bx*2+2, 0)
				si := g.clearBonus(di)
				g.copyRows(si, g.w(vCellAddr+di*2), 4, 0x18)
			}
		}
	}
	for bx := uint16(0); bx < 8; bx += 2 {
		if g.w(vFireFrame+bx) == 0xFF {
			continue
		}
		bp := g.w(vFireCell + bx)
		if bp == g.w(vCell) || (g.b(vDouble) == 1 && bp == g.w(vCell+2)) {
			g.sb(vMaze+bp, g.b(vMaze+bp)&0xFD)
			g.sw(vSndPos, 0)
			g.sw(vSndTune, 0x056F)
			g.sw(vFireFrame+bx, 0xFF)
			g.sw(vEaten, 0)
			for m := uint16(0); m < 4; m++ {
				g.sb(vMonMode+m, 'M')
				g.sw(vMonBlue+m*2, g.random()>>8&0x7F+0xEB)
			}
			g.copyRows(tileDot, g.w(vFireAddr+bx), 4, 0x18)
			// после смены листов кадров BX=6: обход огней на этом тике заканчивается
			bx = g.monsterSprites(sprBlue, 0xFFFF)
			continue
		}
		g.sb(vMaze+bp, g.b(vMaze+bp)|2)
		g.sw(vFireFrame+bx, (g.w(vFireFrame+bx)+4)&0x3F)
		g.copyRows(g.w(0x0530)+g.w(vFireFrame+bx), g.w(vFireAddr+bx), 4, 0x18)
	}
}

// lastFires — L_2a65: на последнем экране второго прохода съеденные огни
// со временем возвращаются.
func (g *Game) lastFires() {
	if g.b(vLastPass) != 1 {
		return
	}
	if g.w(vFireFrame)+g.w(vFireFrame+2)+g.w(vFireFrame+4)+g.w(vFireFrame+6) != 0x3FC {
		return
	}
	if g.w(vFireWait) != 0 {
		g.decw(vFireWait)
		return
	}
	bx := (g.random() >> 8 & 3) << 1
	g.sw(vFireFrame+bx, 0)
	g.sw(vFireWait, g.random()>>8+g.w(vFireStep))
	g.addw(vFireStep, 0x0F)
}

// key — L_2f58: экран 1: ключ появляется в случайной клетке; с ключом
// открываются ворота, второй ключ даёт очки.
func (g *Game) key() {
	if g.w(vHasKey) == 0x0101 {
		return
	}
	if g.b(vKey) == 0 {
		g.keyWait()
		return
	}
	if g.decw(vKeyTime) == 0 {
		g.removeKey()
		return
	}
	bp := g.w(vKeyCell)
	g.sw(vTurn, 0)
	g.sw(vTurn2, 0)
	if bp != g.w(vCell) {
		if g.b(vDouble) != 1 {
			return
		}
		g.sw(vTurn, 1)
		g.sw(vTurn2, 2)
		if bp != g.w(vCell+2) {
			return
		}
	}
	g.sb(vKey, 0)
	bx := g.w(vTurn)
	g.sw(vSndPos, 0)
	g.sw(vSndTune, 0x0583)
	if g.b(vHasKey+bx) == 1 {
		si := g.w(0x0500) + g.w(0x0522)
		g.sw(vKeySpr, si)
		g.copyRows(si, g.w(vKeyAddr), 4, 0x18)
		g.addw(vScore+bx*2, g.w(0x0510)+g.w(0x052A))
		g.drawScore()
		g.sw(vKeyTime, g.random()>>8&0x1F+0x32)
		g.sw(vKeyScore, 0x32)
		return
	}
	g.sb(vHasKey+bx, 1)
	g.drawKeyIcon()
	g.eraseKey()
}

// removeKey — L_3004: ключ исчез, новый появится позже.
func (g *Game) removeKey() {
	g.sb(vKey, 0)
	g.sw(vKeyTime, g.random()>>8&0x1F+0x32)
	g.eraseKey()
}

// eraseKey — L_3017.
func (g *Game) eraseKey() {
	si := g.clearBonus(g.w(vKeyCell))
	g.copyRows(si, g.w(vKeyAddr), 4, 0x18)
	g.sw(vKeyCell, 0)
}

// keyWait — L_303b: ожидание появления ключа.
func (g *Game) keyWait() {
	if g.b(vDouble) == 1 && g.b(vAlive) != 2 {
		if g.b(vHasKey+uint16(g.b(vAlive)^1)) == 1 {
			return
		}
	}
	if g.decw(vKeyTime) != 0 {
		if g.w(vKeyScore) == 0 {
			return
		}
		if g.decw(vKeyScore) == 0 {
			g.eraseKey()
		}
		return
	}
	g.sb(vKey, 1)
	g.sw(vKeyTime, g.random()>>8&0x3F+0xA8)
	bx := g.randomCell()
	g.sw(vKeyCell, bx)
	g.sb(vMaze+bx, g.b(vMaze+bx)|4)
	g.sw(vKeySpr, 0xA996)
	di := g.w(vCellAddr + bx*2)
	g.sw(vKeyAddr, di)
	g.copyRows(0xA996, di, 4, 0x18)
}

// drawKeyIcon — L_30b9: значок ключа на табло (экран 1).
func (g *Game) drawKeyIcon() {
	if g.w(vScreen) != 0 {
		return
	}
	di := uint16(0x4E)
	if g.b(vPlayers) == 1 {
		if g.b(vDouble) == 1 && g.w(vTurn) != 1 {
			di = 0x12
		}
	} else if g.b(vCurPlayer) != 2 {
		di = 0x12
	}
	si := uint16(0xD660 + 0x3F)
	if g.b(vHasKey+g.w(vTurn)) == 1 {
		si = 0xD695
	}
	g.copyRows(si, di, 2, 7)
}

// gate — L_3117: экран 1: игрок с ключом у ворот открывает их; когда в
// воротах никого нет, они закрываются.
func (g *Game) gate() {
	bx := g.w(vTurn)
	if g.b(vHasKey+bx) == 0 {
		return
	}
	switch g.b(vGateAnim) {
	case 'O':
		g.gateOpen()
		return
	case 'C':
		g.gateClose()
		return
	}
	ax := g.w(vCell + g.w(vTurn2))
	in := func(tab uint16) bool {
		for i := uint16(0); i < 5; i++ {
			if ax == g.w(tab+i*2) {
				return true
			}
		}
		return false
	}
	if g.b(vGate) != 'O' {
		if in(0x0546) {
			g.sw(vGateTimer, 1)
			g.gateOpen()
		}
		return
	}
	if g.b(vGateBusy) != 1 && !in(0x0550) {
		return
	}
	inGate := func(c uint16) bool { return c == 0x36 || c == 0x4B || c == 0x60 }
	for m := uint16(0); m < 8; m += 2 {
		if inGate(g.w(vMonCell + m)) {
			g.sb(vGateBusy, 1)
			return
		}
	}
	if inGate(g.w(vCell)) || inGate(g.w(vCell+2)) {
		g.sb(vGateBusy, 1)
		return
	}
	g.gateClose()
}

func (g *Game) gateTimer() bool {
	if g.decw(vGateTimer) != 0 {
		return false
	}
	g.sw(vGateTimer, 2)
	if g.b(vDouble) == 1 && g.w(vHasKey) == 0x0101 {
		g.sw(vGateTimer, 4)
	}
	return true
}

// gateOpen — L_31c6: кадр открывания ворот; открытые пропускают через клетку 4Bh.
func (g *Game) gateOpen() {
	if !g.gateTimer() {
		return
	}
	g.sb(vGateAnim, 'O')
	g.addw(vGateFrame, 6)
	g.gateFrame()
	if g.w(vGateFrame) == 0x30 {
		g.sb(vMaze+0x60, g.b(vMaze+0x60)|0x80)
		g.sb(vMaze+0x36, g.b(vMaze+0x36)|0x40)
		g.sb(vGate, 'O')
		g.sb(vGateAnim, 'N')
	}
}

// gateClose — L_3213: кадр закрывания ворот.
func (g *Game) gateClose() {
	if !g.gateTimer() {
		return
	}
	g.sb(vGateAnim, 'C')
	g.sb(vGateBusy, 0)
	g.sw(vGateFrame, g.w(vGateFrame)-6)
	g.gateFrame()
	g.sb(vMaze+0x60, g.b(vMaze+0x60)&0x7F)
	g.sb(vMaze+0x36, g.b(vMaze+0x36)&0xBF)
	if g.w(vGateFrame) == 0 {
		g.sb(vGate, 'C')
		g.sb(vGateAnim, 'N')
	}
}

// gateFrame — L_3265: кадр ворот 6 байт × 24 строки.
func (g *Game) gateFrame() {
	g.copyRows(g.w(0x0542)+g.w(vGateFrame), g.w(0x0544), 6, 0x18)
}

// stepAnim — общая часть простых анимаций экранов: раз в period+1 тиков
// кадр со страницы 1 (src+frame) выводится в dst; frame растёт на step до end.
func (g *Game) stepAnim(cnt, period, src, frame, dst, step, end uint16, rows int) {
	if g.b(cnt) != 0 {
		g.sb(cnt, g.b(cnt)-1)
		return
	}
	g.sb(cnt, byte(period))
	g.copyRows(g.w(src)+g.w(frame), g.w(dst), 4, rows)
	g.addw(frame, step)
	if g.w(frame) == end {
		g.sw(frame, 0)
	}
}

// anim1a — L_2b45 (экран 1).
func (g *Game) anim1a() { g.stepAnim(0x18, 1, 0x04AA, 0x19, 0x04AC, 4, 0x14, 0x18) }

// anim2b — L_2b78 (экран 2).
func (g *Game) anim2b() { g.stepAnim(0x1F, 1, 0x04B2, 0x20, 0x04B4, 4, 0x50, 0x18) }

// anim2c — L_2bab (экран 2).
func (g *Game) anim2c() { g.stepAnim(0x22, 5, 0x04B6, 0x23, 0x04B8, 4, 0x18, 0x18) }

// anim5 — L_2ab0 (экран 5): 4 байта × 48 строк.
func (g *Game) anim5() { g.stepAnim(0x30, 1, 0x04EC, 0x31, 0x04EE, 4, 0x10, 0x30) }

// anim2a — L_2ae6 (экран 2): кадры 8 байт × 12 строк по кругу вперёд и назад.
func (g *Game) anim2a() {
	if g.b(0x1B) != 0 {
		g.sb(0x1B, g.b(0x1B)-1)
		return
	}
	g.sb(0x1B, 1)
	g.copyRows(g.w(0x04AE)+g.w(0x1D), g.w(0x04B0), 8, 0x0C)
	ax := uint16(0x3C0)
	if g.b(0x1C)&1 != 0 {
		ax = 0xFC48
	}
	g.addw(0x1D, ax)
	if g.b(0x1C) == 0x0A {
		g.sb(0x1C, 0xFF)
		g.sw(0x1D, 0)
	}
	g.sb(0x1C, g.b(0x1C)+1)
}

// anim1b — L_2bde (экран 1): пять анимаций разных размеров.
func (g *Game) anim1b() {
	if g.b(0x25) != 0 {
		g.sb(0x25, g.b(0x25)-1)
		return
	}
	g.sb(0x25, 1)
	for bx := uint16(0); bx < 0x0A; bx += 2 {
		si := g.w(0x04BA+bx) + g.w(0x26+bx)
		dx := g.w(0x04D8 + bx)
		g.copyRows(si, g.w(0x04C4+bx), int(dx), int(g.w(0x04CE+bx)))
		g.addw(0x26+bx, dx)
		if g.w(0x26+bx) == g.w(0x04E2+bx) {
			g.sw(0x26+bx, 0)
		}
	}
}

// drawDots — L_24b8: точки во всех свободных клетках лабиринта экрана scr.
func (g *Game) drawDots(scr uint16) {
	bx := g.w(vMazes+scr) + 0x16
	di := uint16(0x1222)
	for r := 0; r < 11; r++ {
		row := di
		for c := 0; c < 19; c++ {
			if g.b(bx) == ' ' {
				g.copyRows(tileDot, di, 4, 0x18)
			}
			di += 4
			bx++
		}
		bx += 2
		di = row + 0x780
	}
	g.setPalette(palImage)
}

// redrawDots — L_250f: перерисовать клетки по состоянию: точки, пустые
// клетки, фрукт, ключ, ворота, второй CD-Man.
func (g *Game) redrawDots() {
	bx := uint16(0x16)
	di := uint16(0x1222)
	for r := 0; r < 11; r++ {
		row := di
		for c := 0; c < 19; c++ {
			switch {
			case g.b(vMaze+bx)&8 != 0:
				g.copyRows(tileDot, di, 4, 0x18)
			case g.b(vMaze+bx)&1 != 0:
				g.copyRows(tileEmpty, di, 4, 0x18)
			}
			di += 4
			bx++
		}
		bx += 2
		di = row + 0x780
	}
	if g.b(vFruit) != 0 {
		g.copyRows(g.w(vFruitSpr), g.w(vFruitAddr), 4, 0x18)
	}
	if g.b(vKey) == 1 {
		g.copyRows(g.w(vKeySpr), g.w(vKeyAddr), 4, 0x18)
	}
	if g.w(vScreen) == 0 {
		g.gateFrame()
	}
	if g.b(vDouble) == 1 && g.b(vAlive) == 2 {
		ax := g.w(vTurn2)
		t := (ax + 2) & 2
		g.sw(vTurn2, t)
		g.sw(vTurn, t>>1)
		g.drawOther()
		g.sw(vTurn2, ax)
		g.sw(vTurn, ax>>1)
	}
	g.setPalette(palImage)
}

// drawOther — L_0b95: вывести второго CD-Man в режиме Double.
func (g *Game) drawOther() {
	if g.w(vTurn) == 0 {
		if g.w(vLives) == 0 {
			return
		}
		if t := g.b(vTunnel); t < 0x14 && t > 8 {
			g.tunnel()
			return
		}
		g.moveMan()
		return
	}
	g.sw(vTurn, 1)
	g.sw(vTurn2, 2)
	if t := g.b(vTunnel + 1); t < 0x14 && t > 8 {
		g.tunnel()
		return
	}
	g.moveMan()
}

// initMaze — L_25b8: клетки лабиринта экрана scr: точки и разрешённые
// направления; подсчёт точек.
func (g *Game) initMaze(scr uint16) {
	bx := g.w(vMazes+scr) + 0x16
	g.sw(vDots, 0xD1)
	di := uint16(0x00A4)
	for r := 0; r < 11; r++ {
		for c := 0; c < 19; c++ {
			al := byte(0xF9)
			if g.b(bx) != ' ' {
				al &= 0xF6
				g.decw(vDots)
			}
			if g.b(bx-0x15) != ' ' {
				al &= 0x7F
			}
			if g.b(bx+0x15) != ' ' {
				al &= 0xBF
			}
			if g.b(bx-1) != ' ' {
				al &= 0xDF
			}
			if g.b(bx+1) != ' ' {
				al &= 0xEF
			}
			g.sb(di, al)
			bx++
			di++
		}
		bx += 2
		di += 2
	}
	g.sb(vMaze+0x73, g.b(vMaze+0x73)|0x40)
}

// drawScore — L_428e: очки текущего игрока (с нулём в конце) цифрами листа g_102b.
func (g *Game) drawScore() {
	bx := g.w(vTurn2)
	dx := g.w(vScore + bx)
	di := g.w(vScorePos + bx)
	digit := func(al byte) {
		si := g.w(vDigits) + uint16(al)*2
		g.copyRows(si, di, 2, 7)
	}
	for d := uint16(vDivisors); ; d += 2 {
		al := byte(0)
		for int16(dx) >= int16(g.w(d)) {
			al++
			dx -= g.w(d)
		}
		digit(al)
		di += 2
		if g.b(d) == 0x0A {
			break
		}
	}
	digit(byte(dx))
	di += 2
	digit(0)
}

// drawHiScore — L_41cc: надпись «Highscore» и лучший результат.
func (g *Game) drawHiScore() {
	g.copyRows(0xD890+0x34, 1, 0x12, 7)
	saved := g.w(vScore)
	g.sw(vScore, g.w(vTopScores))
	g.sw(vDigits, 0xD890)
	g.sw(vScorePos, 0x0C38)
	g.drawScore()
	g.sw(vScore, saved)
}

// copyLabel — L_420d: надпись 16 байт × 7 строк.
func (g *Game) copyLabel(si, di uint16) {
	g.copyRows(si, di, 0x10, 7)
}

// drawLives — L_4228: значки жизней текущего игрока с позиции di.
func (g *Game) drawLives(di uint16) {
	cx := g.w(vLives)
	if g.b(vPlayers) != 1 {
		cx = g.w(vP1Lives)
		if g.b(vCurPlayer) != 1 {
			cx = g.w(vP2Lives)
		}
	}
	if g.b(vDouble) == 1 && g.w(vTurn) != 0 {
		cx = g.w(vLives + 2)
	}
	g.lifeIcons(0x6DA8, di, cx)
}

// lifeIcons — L_425c: cx значков 4 байта × 18 строк слева направо.
func (g *Game) lifeIcons(si, di, cx uint16) {
	for ; cx > 0; cx-- {
		g.copyRows(si, di, 4, 0x12)
		di += 4
	}
}

// loseLifeIcon — L_3e40: убрать значок жизни, которая сейчас используется.
func (g *Game) loseLifeIcon() {
	bx := uint16(0)
	di := uint16(0x460)
	if g.b(vPlayers)+g.b(vCurPlayer) != 3 {
		if g.b(vDouble) == 1 {
			bx = g.w(vTurn2)
		}
		if !(g.b(vDouble) == 1 && bx == 0) {
			di = 0x49C
		}
	}
	di += (g.w(vLives+bx) - 1) << 2
	g.lifeIcons(0x6DAC, di, 1)
}
