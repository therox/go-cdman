package game

// Переменные этапа.
const (
	vScreen     = 0x0466 // экран (тема) × 2: 0..8
	vRound      = 0x0468 // круг: 1, 2 (повтор экрана), 3 (второй проход)
	vLastPass   = 0x046A // последний экран второго прохода
	vStartScr   = 0x027B // экран, с которого начинается игра
	vPause      = 0x027A
	vDots       = 0x0078 // сколько точек осталось
	vDifficulty = 0x024E // порог случайности монстров
	vLives      = 0x0246 // жизни игрока 1 (и 2 в режиме Double — 0248h)
	vRedraw     = 0x0270 // экран затёрт (после WOSHI) — перезагрузить
	vTheme      = 0x021D // данные темы
	vThemes     = 0x0D93 // таблица данных тем
	vFilesGfx   = 0x0479 // GRAFIXn → страница 2
	vFilesSheet = 0x046F // GRAPHICn → страница 1
	vFilesBack  = 0x0483 // SCREENn → страница 0
	vProtect    = 0x0271 // защита от копирования ещё не показана
	vSwitch     = 0x0076 // в игре на двоих — передать ход
	vDigits     = 0x102B // лист цифр очков
	vScorePos   = 0x102D // место очков игроков на экране
)

// playOne — L_0285: игра одного игрока: пять экранов по два круга, затем
// третий круг по всем экранам.
func (g *Game) playOne() {
	g.sw(vScreen, 0)
	g.sw(vRound, 1)
	g.sb(vLastPass, 0)
	g.sb(vInMenu, 0)
	g.sb(vPause, 0)
	g.sw(vScreen, g.w(vStartScr))
	for {
		g.loadScreen()
		g.saveState(segP1)
		if g.w(vRound) != 3 {
			g.sw(vRound, 1)
		}
		for {
			g.levelHook()
			g.initLevel()
			g.drawHUD()
			g.gameTime()
			for {
				g.tick()
				if g.b(vNewGame) == 1 {
					g.sb(vInMenu, 1)
					return
				}
				g.animate()
				if g.w(vDots) == 0 {
					break
				}
			}
			g.gameTime()
			if g.b(vQuit) == 1 || g.b(vDemoStop) == 1 {
				g.sb(vInMenu, 1)
				return
			}
			if g.w(vLives) == 0 {
				g.sb(vAfterGame, 1)
				g.sb(vInMenu, 1)
				return
			}
			g.sb(vDifficulty, g.b(vDifficulty)-0x10)
			g.restoreState(segP1)
			if g.b(vDemo) == 1 || g.w(vRound) != 1 {
				break
			}
			g.incw(vRound)
			g.drawDots(g.w(vScreen))
		}
		if g.b(vDemo) == 1 && g.w(vScreen) == 2 {
			g.demoEnded()
			return
		}
		if g.nextScreen(vScreen, vRound, vLastPass) {
			g.sb(vAfterGame, 1)
			g.sb(vInMenu, 1)
			return
		}
	}
}

// nextScreen — переход к следующему экрану; true — игра пройдена.
func (g *Game) nextScreen(scr, round, pass uint16) bool {
	g.addw(scr, 2)
	if scr != vScreen {
		g.addw(vScreen, 2)
	}
	if g.w(scr) == 8 && g.w(round) == 3 {
		g.sb(pass, 1)
		g.sb(vLastPass, 1)
	}
	if g.w(scr) != 0x0A {
		return false
	}
	if g.w(round) == 3 {
		return true
	}
	g.sw(scr, 0)
	g.sw(vScreen, 0)
	g.sw(round, 3)
	g.sw(vRound, 3)
	return false
}

// Переменные игры на двоих по очереди.
const (
	vP1Screen = 0x023C
	vP1Round  = 0x023E
	vP2Screen = 0x0240
	vP2Round  = 0x0242
	vP1Pass   = 0x0244
	vP2Pass   = 0x0245
	vP1Lives  = 0x024A
	vP2Lives  = 0x024C
	vP1Diff   = 0x024F
	vP2Diff   = 0x0250
	vP1Score  = 0x0237
	vP2Score  = 0x0239
)

// Метки игры на двоих (адреса оригинала).
const (
	l0390 = iota
	l0393
	l03a6
	l03b5
	l044f
	l0465
	l0474
)

// playTwo — L_036b: два игрока по очереди; после гибели ход переходит к
// другому игроку и продолжается с того места, где он остановился.
func (g *Game) playTwo() {
	g.sb(vInMenu, 0)
	g.sb(vCurPlayer, 1)
	g.sw(vScreen, 0)
	g.loadScreen()
	g.initLevel()
	g.saveState(segP4)
	g.saveState(segP2)
	lbl := l0393
	for {
		switch lbl {
		case l0390:
			g.loadScreen()
			lbl = l0393
		case l0393:
			g.saveState(segP1)
			if g.w(vP1Round) != 3 {
				g.sw(vP1Round, 1)
			}
			lbl = l03a6
		case l03a6:
			g.levelHook()
			g.initLevel()
			g.drawHUD()
			g.gameTime()
			g.sw(vRound, g.w(vP1Round))
			lbl = l03b5
		case l03b5:
			g.tick()
			if g.b(vNewGame) == 1 {
				return
			}
			if g.b(vSwitch) == 1 {
				g.switchToP2()
				lbl = l0474
				continue
			}
			g.animate()
			if g.w(vDots) != 0 {
				continue
			}
			g.gameTime()
			if g.b(vQuit) == 1 {
				return
			}
			g.restoreState(segP1)
			if g.w(vP1Round) == 1 {
				g.incw(vP1Round)
				g.drawDots(g.w(vP1Screen))
				lbl = l03a6
				continue
			}
			g.addw(vP1Screen, 2)
			g.addw(vScreen, 2)
			if g.w(vP1Screen) == 8 && g.w(vP1Round) == 3 {
				g.sb(vP1Pass, 1)
				g.sb(vLastPass, 1)
			}
			if g.w(vP1Screen) == 0x0A {
				if g.w(vP1Round) == 3 {
					g.p1Over()
					return
				}
				g.sw(vP1Screen, 0)
				g.sw(vScreen, 0)
				g.sw(vP1Round, 3)
				g.sw(vRound, 3)
			}
			lbl = l0390
		case l044f:
			g.loadScreen()
			g.saveState(segP2)
			if g.w(vP2Round) != 3 {
				g.sw(vP2Round, 1)
			}
			lbl = l0465
		case l0465:
			g.levelHook()
			g.initLevel()
			g.drawHUD()
			g.gameTime()
			g.sw(vRound, g.w(vP2Round))
			lbl = l0474
		case l0474:
			g.tick()
			if g.b(vNewGame) == 1 {
				return
			}
			if g.b(vSwitch) == 1 {
				g.switchToP1()
				if g.b(vInMenu) == 1 {
					return
				}
				lbl = l03b5
				continue
			}
			g.animate()
			if g.w(vDots) != 0 {
				continue
			}
			g.gameTime()
			if g.b(vQuit) == 1 {
				return
			}
			g.restoreState(segP2)
			if g.w(vP2Round) == 1 {
				g.incw(vP2Round)
				g.drawDots(g.w(vP2Screen))
				lbl = l0465
				continue
			}
			g.addw(vP2Screen, 2)
			g.addw(vScreen, 2)
			if g.w(vP2Screen) == 8 && g.w(vP2Round) == 3 {
				g.sb(vP2Pass, 1)
				g.sb(vLastPass, 1)
			}
			if g.w(vP2Screen) == 0x0A {
				if g.w(vP2Round) == 3 {
					g.p2Over()
					return
				}
				g.sw(vP2Screen, 0)
				g.sw(vScreen, 0)
				g.sw(vP2Round, 3)
				g.sw(vRound, 3)
			}
			lbl = l044f
		}
	}
}

// p1Over — L_0516: игра первого игрока окончена (ввод имени через меню).
func (g *Game) p1Over() {
	g.winner()
	g.sb(vAfterGame, 1)
	g.sb(vEnding, 1)
	g.escMenu()
}

// p2Over — L_0527: игра второго игрока окончена.
func (g *Game) p2Over() {
	g.winner()
	g.sb(vAfterGame, 1)
	g.sb(vInMenu, 1)
}

// switchToP2 — L_0535: передать ход второму игроку.
func (g *Game) switchToP2() {
	g.sb(vSwitch, 0)
	g.gameTime()
	g.saveState(segP3)
	if g.w(vP1Lives) == 1 {
		g.p1Over()
	}
	g.decw(vP1Lives)
	g.sw(vLives, g.w(vP2Lives))
	g.sb(vLastPass, g.b(vP2Pass))
	g.sb(vP1Diff, g.b(vP1Diff)-0x10)
	g.sb(vDifficulty, g.b(vP2Diff))
	g.sw(vP1Score, g.w(vScore))
	g.sw(vScore, g.w(vP2Score))
	g.sb(vCurPlayer, 2)
	g.gameTime()
	g.sw(vScreen, g.w(vP2Screen))
	if g.w(vScreen) != g.w(vP1Screen) {
		g.loadScreen()
	}
	g.reshow()
	g.restoreState(segP4)
	g.redrawDots()
	g.drawHUD()
}

// switchToP1 — L_059b: передать ход первому игроку (или конец игры, если
// у второго это была последняя жизнь).
func (g *Game) switchToP1() {
	g.sb(vSwitch, 0)
	g.gameTime()
	g.saveState(segP4)
	if g.w(vP2Lives) == 1 {
		g.p2Over()
		return
	}
	g.decw(vP2Lives)
	g.sw(vLives, g.w(vP1Lives))
	g.sb(vLastPass, g.b(vP1Pass))
	g.sb(vP2Diff, g.b(vP2Diff)-0x10)
	g.sb(vDifficulty, g.b(vP1Diff))
	g.sw(vP2Score, g.w(vScore))
	g.sw(vScore, g.w(vP1Score))
	g.sb(vCurPlayer, 1)
	g.gameTime()
	g.sw(vScreen, g.w(vP1Screen))
	if g.w(vScreen) != g.w(vP2Screen) {
		g.loadScreen()
	}
	g.reshow()
	g.restoreState(segP3)
	g.redrawDots()
	g.drawHUD()
}

// playDouble — L_0601: режим Double — два CD-Man одновременно.
func (g *Game) playDouble() {
	g.sw(vScreen, 0)
	g.sw(vRound, 1)
	g.sb(vLastPass, 0)
	g.sb(vInMenu, 0)
	g.sb(vAlive, 2)
	g.sb(0x003F, 0)
	g.sb(vPause, 0)
	g.sw(vScreen, g.w(vStartScr))
	for {
		g.loadScreen()
		g.saveState(segP1)
		if g.w(vRound) != 3 {
			g.sw(vRound, 1)
		}
		for {
			g.levelHook()
			g.initLevel()
			g.drawHUD()
			g.gameTime()
			for {
				g.tick()
				if g.b(vNewGame) == 1 {
					g.sb(vInMenu, 1)
					return
				}
				g.animate()
				if g.w(vLives)+g.w(vLives+2) == 0 {
					g.doubleOver()
					return
				}
				if g.w(vDots) == 0 {
					break
				}
			}
			g.gameTime()
			if g.b(vQuit) == 1 {
				g.sb(vInMenu, 1)
				return
			}
			g.sb(vDifficulty, g.b(vDifficulty)-0x10)
			g.restoreState(segP1)
			if g.b(vDemo) == 1 || g.w(vRound) != 1 {
				break
			}
			g.incw(vRound)
			g.drawDots(g.w(vScreen))
		}
		if g.nextScreen(vScreen, vRound, vLastPass) {
			g.doubleOver()
			return
		}
	}
}

// doubleOver — L_06d9: конец игры Double; имя вводит тот, кто жил дольше.
func (g *Game) doubleOver() {
	g.sb(vAfterGame, 1)
	g.sb(vLastWhich, 0)
	g.sb(vPlayers, 2)
	al := uint16(g.b(vAlive))
	g.sw(vTurn, 1-al)
	g.sw(vTurn2, g.w(vTurn2)-al<<1)
	g.sb(vCurPlayer, byte(g.w(vTurn)+1))
	g.gameTime()
	g.sb(vInMenu, 1)
}

// loadScreen — L_070d: загрузить тему экрана: GRAFIXn на страницу 2,
// GRAPHICn на страницу 1, SCREENn на страницу 0 (при чёрной палитре);
// при первом запуске — защита от копирования; точки и клетки лабиринта.
func (g *Game) loadScreen() {
	g.sb(vRedraw, 0)
	bx := g.w(vScreen)
	g.sw(vTheme, g.w(vThemes+bx))
	g.loadFile(g.w(vFilesGfx + bx))
	g.decodeImage(0xADAC)
	g.loadFile(g.w(vFilesSheet + bx))
	g.decodeImage(0xA6D6)
	g.loadFile(g.w(vFilesBack + bx))
	if g.b(vDemo) != 1 && g.b(vProtect) == 1 {
		g.sb(vProtect, 0)
		g.protection()
	}
	g.setPalette(palBlack)
	g.decodeImage(0xA000)
	g.drawDots(g.w(vScreen))
	g.initMaze(g.w(vScreen))
	g.v.OutGC(0x0105)
}

// protection — L_4318: защита от копирования — огонь и вопрос о его имени из
// руководства. В порте защита не показывается; здесь повторяются только её
// следы в состоянии программы (палитра, задержка, номер огня, шаг генератора).
func (g *Game) protection() {
	g.setPalette(palBlack)
	g.clearPage0()
	g.setPalette(palImage)
	g.sw(vDelayMul, 0x32)
	ax := g.random()
	g.sw(0x3395, ax>>8&0x0F)
}

// reshow — L_077f: после экрана WOSHI заново загрузить фон (в режиме Double —
// и табло обоих игроков); вернуть палитру картинки.
func (g *Game) reshow() {
	if g.b(vRedraw) == 1 {
		g.sb(vRedraw, 0)
		g.loadFile(g.w(vFilesBack + g.w(vScreen)))
		g.setPalette(palBlack)
		g.decodeImage(0xA000)
		if g.b(vDouble) == 1 {
			g.redrawDots()
			g.sw(vTurn, 0)
			g.sw(vTurn2, 0)
			g.drawLives(0x460)
			g.drawKeyIcon()
			g.copyLabel(0xD8A4, 0)
			g.sw(vDigits, 0xD890)
			g.sw(vScorePos, 0x0C38)
			g.drawScore()
			g.sw(vTurn, 1)
			g.sw(vTurn2, 2)
			g.drawLives(0x49C)
			g.drawKeyIcon()
			g.copyLabel(0xD8B4, 0x3C)
			g.sw(vScorePos+2, 0x0C6C)
			g.drawScore()
		}
	} else if g.b(vDouble) != 1 {
		return
	}
	g.setPalette(palImage)
	g.v.OutGC(0x0105)
}

// drawHUD — L_0827: табло в начале этапа и мигание надписи «Player».
func (g *Game) drawHUD() {
	switch {
	case g.b(vPlayers) == 2:
		g.drawHUDTwo()
	case g.b(vDouble) == 1:
		g.sw(vTurn, 0)
		g.drawLives(0x460)
		g.drawKeyIcon()
		g.sw(vTurn, 1)
		g.drawLives(0x49C)
		g.drawKeyIcon()
		g.startDouble()
	default:
		g.drawHiScore()
		g.drawKeyIcon()
		g.drawLives(0x49C)
		g.sw(vScorePos, 0x0C6C)
		g.moveMan()
		g.timers()
		g.sw(vScorePos, 0x0C6C)
		g.blinkLabel(0x3C, 5, 0x14)
		g.loseLifeIcon()
	}
}

// drawHUDTwo — L_086a: табло игры на двоих.
func (g *Game) drawHUDTwo() {
	si, di := uint16(0xD890+0x14), uint16(0)
	if g.b(vCurPlayer) == 1 {
		si += 0x10
		di += 0x3C
	}
	g.copyLabel(si, di)
	g.drawKeyIcon()
	cur := g.b(vCurPlayer)
	g.sb(vCurPlayer, 1)
	g.drawLives(0x460)
	g.sb(vCurPlayer, 2)
	g.drawLives(0x49C)
	g.sb(vCurPlayer, cur)
	score := g.w(vScore)
	g.sw(vDigits, 0xD890)
	g.sw(vScorePos, 0x0C38)
	ax := g.w(vP1Score)
	if g.b(vCurPlayer) == 1 {
		g.addw(vScorePos, 0x34)
		ax = g.w(vP2Score)
	}
	g.sw(vScore, ax)
	other := g.w(vScore)
	g.moveMan()
	g.sw(vScore, other)
	g.drawScore()
	g.sw(vScore, score)
	g.timers()
	di, dx := uint16(0), uint16(0x14)
	g.sw(vScorePos, 0x0C38)
	if g.b(vCurPlayer) != 1 {
		di += 0x3C
		dx += 0x10
		g.addw(vScorePos, 0x34)
	}
	g.blinkLabel(di, 0xC8, dx)
	g.loseLifeIcon()
}

// startDouble — L_0924: начало этапа Double.
func (g *Game) startDouble() {
	g.timers()
	if g.b(vAlive) != 0 {
		g.sw(vTurn, 0)
		g.sw(vTurn2, 0)
		g.moveMan()
	}
	if g.b(vAlive) != 1 {
		g.sw(vTurn, 1)
		g.sw(vTurn2, 2)
		g.moveMan()
	}
	g.sw(vScorePos, 0x0C38)
	g.sw(vScorePos+2, 0x0C6C)
	g.sw(vScorePos, 0x0C38)
	g.blinkLabel(0, 0x0A, 0x14)
	if g.b(vAlive) != 0 {
		g.sw(vTurn2, 0)
		g.loseLifeIcon()
	}
	if g.b(vAlive) != 1 {
		g.sw(vTurn2, 2)
		g.loseLifeIcon()
	}
}

// blinkLabel — L_0992: мигание надписи «Player» и очков cx раз (пока не
// нажата клавиша; нажатие остаётся в буфере).
func (g *Game) blinkLabel(di, cx, dx uint16) {
	for {
		g.call(0x0992)
		show := func(sheet uint16) {
			p1 := !(g.b(vDouble) == 1 && g.b(vAlive) == 0)
			if p1 {
				g.copyLabel(sheet+dx, di)
				g.sw(vDigits, sheet)
				g.drawScore()
			}
			if !p1 || (g.b(vDouble) == 1 && g.b(vAlive) != 1) {
				g.copyLabel(sheet+0x24, 0x3C)
				g.sw(vDigits, sheet)
				g.sw(vTurn2, 2)
				g.drawScore()
				g.sw(vTurn2, 0)
			}
		}
		show(0xD890)
		g.sw(vDelayMul, 0x96)
		g.waitNKey()
		show(0xD660)
		if g.b(vSkipKey) == 1 {
			return
		}
		g.sw(vDelayMul, 0x96)
		g.waitNKey()
		if g.b(vSkipKey) == 1 {
			return
		}
		cx--
		if cx == 0 {
			return
		}
	}
}

// initLevel — L_0a60: начальные положения и направления из данных темы.
func (g *Game) initLevel() {
	si := g.w(vTheme)
	copy(g.ds[0x19F:0x19F+0x66], g.ds[si:si+0x66])
	g.monsterSprites(0x0FF5, 0xFFFF)
	if g.b(vDouble) == 0 {
		return
	}
	copy(g.ds[0x19F:0x19F+0x12], g.ds[si+0x66:si+0x66+0x12])
	if g.b(vAlive) != 2 {
		bx := uint16(g.b(vAlive)) << 1
		g.sw(0x01AD+bx, 0)
		g.sw(0x01A5+bx, 0)
	}
}

// monsterSprites — L_2482: листы кадров монстра bx (0FFFFh — всех четырёх)
// из таблицы DS:si.
func (g *Game) monsterSprites(si, bx uint16) uint16 {
	n := 1
	if bx == 0xFFFF {
		bx, n = 0, 4
	}
	for ; n > 0; n-- {
		g.sw(0x0205+bx, g.w(si))
		g.sw(0x020D+bx, g.w(si+2))
		g.sw(0x0215+bx, g.w(si+4))
		bx += 2
	}
	return bx - 2
}
