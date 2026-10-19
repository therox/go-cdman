package game

// Листы кадров монстров.
const (
	sprNormal = 0x0FF5 // обычные
	sprBlue   = 0x0FFB // синие
	sprEnding = 0x1001 // синие, скоро станут обычными
)

// thinkMonster — L_27ed: на подшаге 7 монстр выбирает направление в
// следующей клетке: в случайном порядке либо (с вероятностью, растущей от
// этапа к этапу) к своему CD-Man, а синий — от него. Развернуться назад
// можно только из тупика; занятые огнями и другими монстрами клетки
// пропускаются.
func (g *Game) thinkMonster() {
	bx := g.w(vCurMon)
	if g.b(vMonSub+bx) != 7 {
		return
	}
	if g.w(vMonWait+bx*2) >= 0x11 {
		return
	}
	var ah, al, bh, bl byte
	r := g.random()
	if byte(r>>8) > g.b(vDifficulty) {
		// L_2874: к цели (синий — от неё)
		col, row := cellRowCol(g.w(vMonCell + bx*2))
		ah, al, bh, bl = 1, 3, 0, 2
		if g.b(vMonMode+bx) != 'G' {
			g.sw(vStep, 0x0103)
			ah, al, bh, bl = 0, 2, 1, 3
		}
		si := (bx & 1) << 1
		dx := int16(col - g.w(vTargetX+si))
		if dx < 0 {
			g.sb(vTmp, ah)
			ah, bh = bh, ah
			dx = -dx
		}
		dy := int16(row - g.w(vTargetY+si))
		if dy < 0 {
			g.sb(vTmp, al)
			al, bl = bl, al
			dy = -dy
		}
		if uint16(dx) > uint16(dy) {
			g.sb(vTmp, bh)
			bh, bl = bl, bh
		} else {
			g.sb(vTmp, ah)
			ah, al = al, ah
		}
	} else {
		// случайный порядок направлений из битов старшего байта
		rnd := byte(r >> 8)
		list := [4]byte{0, 1, 2, 3}
		take := func(i int) byte {
			v := list[i]
			copy(list[i:], list[i+1:])
			return v
		}
		i := int(rnd&1)<<1 | int(rnd>>1&1)
		ah = take(i)
		rnd >>= 2
		i = int(rnd & 1)
		rnd >>= 1
		if rnd&1 != 0 {
			i++
		}
		al = take(i)
		bh, bl = list[0], list[1]
		if rnd&2 != 0 {
			g.sb(vTmp, bh)
			bh, bl = bl, bh
		}
		copy(g.ds[vPrefs:vPrefs+4], list[:])
	}
	g.sb(vPrefs, ah)
	g.sb(vPrefs+1, al)
	g.sb(vPrefs+2, bh)
	g.sb(vPrefs+3, bl)
	bp := g.w(vMonCell + bx*2)
	choice := byte(0xFF)
	for si := uint16(0); si < 4; si++ {
		d := g.b(vPrefs + si)
		if g.b(vMaze+bp)&(0x10<<d) == 0 {
			continue
		}
		di := g.w(vDeltas + uint16(d)*2)
		if g.b(vMaze+bp+di)&2 != 0 {
			continue
		}
		next := di + bp
		busy := false
		for m := uint16(0); m < 8; m += 2 {
			if g.w(vMonCell+m) == next {
				busy = true
				break
			}
		}
		if busy {
			continue
		}
		dir := d << 3
		if dir^g.b(vMonDir+bx) != 8 {
			choice = dir
			break
		}
		// разворот — только если других выходов нет
		al := g.b(vMaze + bp)
		cl := byte(0xEF)
		for k := uint16(0); k < 4; k++ {
			dd := g.w(vDeltas + k*2)
			n := dd + bp
			if g.b(vMaze+bp+dd)&2 != 0 || n == g.w(0x01BD) || n == g.w(0x01BF) || n == g.w(0x01C1) || n == g.w(0x01C3) {
				al &= cl
			}
			cl = cl<<1 | cl>>7
		}
		exits := 0
		for k := 0; k < 4; k++ {
			if al&0x80 != 0 {
				exits++
			}
			al = al<<1 | al>>7
		}
		if exits == 1 {
			choice = dir
			break
		}
	}
	g.sb(vMonNext+bx, choice)
}

// moveMonster — L_3ad2: шаг монстра и проверка столкновения с CD-Man.
func (g *Game) moveMonster() {
	m := g.w(vCurMon)
	w := m * 2
	if g.w(vMonWait+w) != 0 {
		g.decw(vMonWait + w)
		if g.w(vMonWait+w) > 0x17 {
			return
		}
	}
	if g.b(vMonNext+m) == 0xFF {
		di := g.w(vMonAddr + w)
		if g.b(vMonSub+m)&1 == 0 {
			g.drawMonster5(m, di)
			g.collisions()
			return
		}
		g.drawMonster(m, di)
		g.collisions()
		return
	}
	g.sw(vBack, tileEmpty)
	bp := g.w(vMonFrom + w)
	if g.b(vMonSub+m) == 7 {
		bp = g.w(vMonCell + w)
	}
	if g.b(vMaze+bp)&4 != 0 {
		g.bonusTile(bp)
	} else if g.b(vMaze+bp)&8 != 0 {
		g.sw(vBack, tileDot)
	}
	bp = g.w(vMonCell + w)
	sub := (g.b(vMonSub+m) + 1) & 7
	g.sb(vMonSub+m, sub)
	var dh byte
	if sub != 0 {
		dh = g.b(vMonDir + m)
	} else {
		dh = g.b(vMonNext + m)
		g.sb(vMonDir+m, dh)
		g.sw(vMonFrom+w, bp)
		switch dh {
		case 0:
			g.incw(vMonCell + w)
		case 8:
			g.decw(vMonCell + w)
		case 0x10:
			g.addw(vMonCell+w, 0x15)
		default:
			g.sw(vMonCell+w, g.w(vMonCell+w)-0x15)
		}
	}
	odd := g.b(vMonSub+m)&1 != 0
	switch dh {
	case 0: // вправо
		di := g.w(vMonAddr + w)
		if !odd {
			g.drawMonster5(m, di)
			g.collisions()
			return
		}
		g.monEraseLeft(m, di)
		di++
		g.sw(vMonAddr+w, di)
	case 8: // влево
		di := g.w(vMonAddr + w)
		if !odd {
			di--
			g.sw(vMonAddr+w, di)
			g.drawMonster5(m, di)
			g.collisions()
			return
		}
		g.monEraseRight(m, di)
	case 0x10: // вниз
		di := g.w(vMonAddr+w) + 0xF0
		if g.w(vMonWait+w) <= 0x0F {
			di -= 0xF0
			g.monEraseAbove(m, di)
			di += 0xF0
		}
		g.sw(vMonAddr+w, di)
	default: // вверх
		di := g.w(vMonAddr+w) + 0x690
		g.monEraseBelow(m, di)
		di -= 0x870 - 0xF0
		g.sw(vMonAddr+w, di)
	}
	g.drawMonster(m, g.w(vMonAddr+w))
	g.collisions()
}

// collisions — L_3c42: проверить столкновение монстра с каждым CD-Man.
func (g *Game) collisions() {
	cx := g.w(vManAddr)
	g.sw(vTurn, 0)
	g.sw(vTurn2, 0)
	g.collide(cx)
	if g.b(vDouble) == 1 {
		cx = g.w(vManAddr + 2)
		g.sw(vTurn, 1)
		g.sw(vTurn2, 2)
		g.collide(cx)
	}
}

// collide — L_3c70: монстр задевает CD-Man (адрес cx), если их адреса на
// странице близки; обычный монстр губит CD-Man, синий — съедается.
func (g *Game) collide(cx uint16) {
	m := g.w(vCurMon)
	ax := g.w(vMonAddr+m*2) + 0x693 - cx
	if ax > 0xD20 {
		return
	}
	hit := ax <= 6 || ax == 0x690 || ax == 0x696
	if !hit {
		dx := uint16(0xF1)
		for i := 0; i < 0x0E; i++ {
			if ax < dx {
				return
			}
			dx += 4
			if ax < dx {
				hit = true
				break
			}
			dx += 0xEC
		}
		if !hit {
			return
		}
	}
	if g.b(vMonMode+m) != 'M' {
		g.die()
		return
	}
	g.eatMonster()
}

// drawMonster — L_3fac: кадр монстра шириной 4 байта (при выходе из клетки
// монстров рисуется только нижняя часть).
func (g *Game) drawMonster(m, di uint16) {
	w := m * 2
	dl := uint16(g.b(vMonSub + m))
	si := g.w(0x0205 + w)
	al := g.b(vMonDir + m)
	if al&0x10 != 0 {
		si += 0x20
		if dl&1 == 0 {
			si = g.w(0x020D + w)
		}
	}
	si += (dl &^ 1) << 1
	si += uint16(al&8) << 1
	if g.w(vMonWait+w) <= 0x10 {
		g.copyRows(si, di, 4, 0x18)
		return
	}
	rows := 0x18
	cl := 8 - (int(g.b(vMonSub+m)) + 1)
	if cl == 0 {
		// «loop» с CX=0 — 65536 итераций, после которых всё на прежнем месте
		g.spin(0x10000 * 4)
	}
	for ; cl > 0; cl-- {
		rows -= 3
		si += 0xF0
		di += 0xF0
	}
	g.copyRows(si, di, 4, rows&0xFF)
}

// drawMonster5 — L_405e: кадр шириной 5 байт (сдвиг на полбайта).
func (g *Game) drawMonster5(m, di uint16) {
	d := uint16(g.b(vMonDir + m))
	cx := d + d + d>>1
	dl := uint16(g.b(vMonSub+m) &^ 1)
	cx += dl + dl + dl>>1
	si := g.w(0x0215+m*2) + cx
	g.copyRows(si, di, 5, 0x18)
}

// monEraseBelow — L_40b9.
func (g *Game) monEraseBelow(m, di uint16) {
	si := g.w(vBack) + 0x780
	for n := int(g.b(vMonSub+m)) + 1; n > 0; n-- {
		si -= 0xF0
	}
	g.copyRows(si, di, 4, 3)
}

// monEraseAbove — L_40f9.
func (g *Game) monEraseAbove(m, di uint16) {
	si := g.w(vBack) - 0xF0
	for n := int(g.b(vMonSub+m)) + 1; n > 0; n-- {
		si += 0xF0
	}
	g.copyRows(si, di, 4, 3)
}

// monEraseLeft — L_4139.
func (g *Game) monEraseLeft(m, di uint16) {
	g.copyRows(g.w(vBack)+uint16(g.b(vMonSub+m)>>1), di, 1, 0x18)
}

// monEraseRight — L_415d.
func (g *Game) monEraseRight(m, di uint16) {
	g.copyRows(g.w(vBack)+3-uint16(g.b(vMonSub+m)>>1), di+4, 1, 0x18)
}

// die — L_3cdb: монстр поймал CD-Man: монстры возвращаются в клетку,
// мелодия и кадры гибели; потеря жизни или конец игры.
func (g *Game) die() {
	g.redrawDots()
	si := g.w(vThemes + g.w(vScreen))
	g.sw(vTheme, si)
	copy(g.ds[vMonDir:vMonDir+0x2C], g.ds[si+0x12:si+0x12+0x2C])
	g.monsterSprites(sprNormal, 0xFFFF)
	g.sw(vSndPos, 0)
	g.sw(vSndTune, 0x05AB)
	si = 0xB0E0
	t2 := g.w(vTurn2)
	di := g.w(vManAddr + t2)
	p := t2 >> 1
	if g.b(vInTunnel+p) != 0 {
		d := g.w(vTunnelEnd + p*2)
		if g.b(vTunnel+p) <= 0x0A {
			d = (d + 2) & 2
		}
		di = g.w(0x01F5 + d + 4)
	}
	for cx := 0x15; cx > 0; cx-- {
		g.copyRows(si, di, 4, 0x18)
		g.sound()
		g.doorAnim()
		g.sw(vDelayMul, 0x14)
		g.waitN()
		if cx <= 0x10 {
			si += 4
		}
	}
	g.sw(vDelayMul, 0x5DC)
	g.waitNKey()
	si = tileEmpty
	if g.b(vMaze+g.w(vCell+g.w(vTurn2)))&8 != 0 {
		si = tileDot
	}
	g.copyRows(si, di, 4, 0x18)
	if g.b(vFruit) != 0 {
		g.copyRows(g.w(vFruitSpr), g.w(vFruitAddr), 4, 0x18)
	}
	if g.b(vPlayers) == 2 {
		g.sb(vSwitch, 1)
		return
	}
	bx := g.w(vTurn2)
	if g.b(vPause) != 1 {
		g.decw(vLives + bx)
		if g.w(vLives+bx) == 0 {
			if g.b(vDouble) == 0 || g.b(vAlive) != 2 {
				g.sw(vDots, 0)
				g.sb(vStop, 1)
				g.winner()
				return
			}
			// в режиме Double первый выбывший вводит имя, второй играет дальше
			g.sw(vManAddr+bx, 0)
			g.sw(vCell+bx, 0)
			bx >>= 1
			g.sb(vAlive, byte(bx))
			g.sb(vHasKey+bx, 1)
			g.removeKey()
			g.winner()
			g.sb(vAfterGame, 1)
			g.sb(vLastWhich, 1)
			g.sb(vPlayers, 2)
			g.sb(vCurPlayer, byte(g.w(vTurn)+1))
			g.sb(vEnding, 1)
			g.escMenu()
			g.sb(vPlayers, 1)
			g.sb(vCurPlayer, 1)
			g.reshow()
		}
	}
	if g.b(vDouble) == 1 {
		g.startDouble()
		bx = 0
		if g.b(vAlive) != 1 {
			bx = 2
		}
	} else {
		g.drawHUD()
		bx = 0
	}
	if g.w(vLives+bx) != 0 {
		g.sw(vDelayMul, 0xBB8)
		g.waitNKey()
	}
}

// eatMonster — L_3e79: CD-Man съел синего монстра: очки (200, 400, 800,
// 1600), монстр возвращается в клетку.
func (g *Game) eatMonster() {
	di := uint16(vStats1 + 2)
	if g.b(vCurPlayer) == 2 || g.w(vTurn) == 1 {
		di = vStats2 + 2
	}
	g.incw(di)
	g.sw(vSndPos, 0)
	g.sw(vSndTune, 0x05D7)
	m := g.w(vCurMon)
	g.sb(vMonMode+m, 'G')
	w := m * 2
	g.sw(vMonBlue+w, 0)
	g.monsterSprites(sprNormal, w)
	r := g.random()
	g.sw(vMonWait+w, r>>8+0x64)
	di = g.w(vMonAddr + w)
	if g.b(vMonSub+m)&1 == 0 && g.b(vMonDir+m)&0x10 == 0 {
		g.copy5(tileEmpty, di)
	} else {
		g.copyRows(tileEmpty, di, 4, 0x18)
	}
	g.sw(vMonScore+w, 0x32)
	k := g.w(vEaten)
	dx := g.w(0x053A + k)
	si := g.w(0x0532 + k)
	di = g.w(vMonCell + w)
	old := g.w(vBonus + w*2)
	g.sb(vMaze+old, g.b(vMaze+old)&0xFB)
	g.sw(vBonus+w*2, di)
	g.sw(vBonus+w*2+2, si)
	g.sb(vMaze+di, g.b(vMaze+di)|4)
	g.copyRows(si, g.w(vCellAddr+di*2), 4, 0x18)
	if from := g.w(vMonFrom + w); g.b(vMaze+from)&8 != 0 {
		g.copyRows(tileDot, g.w(vCellAddr+from*2), 4, 0x18)
	}
	// направления и место — как у монстра 0 в начале этапа
	src := g.w(vTheme) + 0x12
	for i := uint16(0); i < 3; i++ {
		g.sb(vMonDir+m+i*4, g.b(src+i*4))
	}
	src += 12
	for i := uint16(0); i < 3; i++ {
		g.sw(vMonCell+w+i*8, g.w(src+i*8))
	}
	g.addw(vScore+g.w(vTurn2), dx)
	g.drawScore()
	g.addw(vEaten, 2)
	if g.w(vEaten) != 8 {
		return
	}
	g.sw(vEaten, 0)
	for bp := uint16(0); bp < 4; bp++ {
		g.sb(vMonMode+bp, 'G')
		g.sw(vMonBlue+bp*2, 0)
	}
	g.monsterSprites(sprNormal, 0xFFFF)
}

// copy5 — L_29df: 5 байт × 24 строки, пятый байт — копия четвёртого байта источника.
func (g *Game) copy5(si, di uint16) {
	for r := uint16(0); r < 0x18; r++ {
		for k := uint16(0); k < 4; k++ {
			g.v.Copy(si+k, di+k)
		}
		g.v.Copy(si+3, di+4)
		si += 80
		di += 80
	}
	g.spin(0x18 * 13)
}
