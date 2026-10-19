package game

// Сообщения и окна меню.
const (
	msgNoHelp     = 0x12FF // «I can't find HELP.CDM file»
	msgHelpFirst  = 0x1321 // подсказки листания справки
	msgHelpMiddle = 0x1347
	msgHelpLast   = 0x1370
	msgDemoGame   = 0x10C6 // «Demo will corrupt your current game…»
	msgNoJoy      = 0x1237 // «I can't detect a Joystick…»
	msgCenterJoy  = 0x1278 // «Center your Joystick…»
	msgKeyboard   = 0x12AF // «Press Enter for Keyboard Control»
	msgJoystick   = 0x12D7 // «Press Enter for Joystick Control»
	vYesNo        = 0x1071 // ответ окна Yes/No
	vYesNoSave    = 0x1072 // под окном Yes/No
	vYesNoBox     = 0x10AA
	vControl      = 0x110F // управление: «K» клавиатура, «J» джойстик
	vControlOld   = 0x1110
	vControlPos   = 0x110D
	vControlSave  = 0x1111
	vControlBox   = 0x11D5
)

// help — L_14c8: справка из HELP.CDM постранично (стрелки вверх и вниз).
func (g *Game) help() {
	g.loadFile(fnHelp)
	if g.b(vNoHelp) != 0 {
		g.message(msgNoHelp)
		return
	}
	buf := g.buf[g.bufBase():]
	si, bx := g.helpPage(0)
	for {
		ax := g.getKey()
		al, ah := byte(ax), byte(ax>>8)
		if al == asciiEsc || al == asciiEnter {
			return
		}
		if al != 0 {
			continue
		}
		switch ah {
		case scanUp:
			if bx == 0 {
				continue
			}
			si = bx
			found := false
			for cx := 0x1A; ; {
				si--
				if si == 0 {
					found = true
					break
				}
				if buf[si] != 0x0D {
					continue
				}
				cx--
				if cx == 0 {
					break
				}
			}
			if !found {
				si += 2
			}
			si, bx = g.helpPage(si)
		case scanDown:
			if buf[si] == 0x1A {
				continue
			}
			si, bx = g.helpPage(si)
		}
	}
}

// helpPage — L_1531: страница справки с позиции si буфера (строка-заголовок
// белым, 24 строки красным) и подсказка листания. Возвращает начало
// следующей страницы и начало показанной.
func (g *Game) helpPage(si int) (next, start int) {
	buf := g.buf[g.bufBase():]
	g.clearText()
	start = si
	line := func(di uint16, ah byte) {
		for {
			al := buf[si]
			si++
			if al == 0x0D {
				break
			}
			g.v.TextW(di, uint16(ah)<<8|uint16(al))
			di += 2
		}
		si++
	}
	line(0, 1)
	di := uint16(0xA0)
	for i := 0; i < 0x18; i++ {
		line(di, 4)
		di += 0xA0
	}
	msg := uint16(msgHelpFirst)
	if start != 0 {
		msg = msgHelpMiddle
		if buf[si] == 0x1A {
			msg = msgHelpLast
		}
	}
	g.message(msg)
	g.v.SetCursor(0x1A00)
	return si, start
}

// startDemo — L_15b4: начать демонстрацию (во время игры — с подтверждением).
func (g *Game) startDemo() {
	if g.b(vInGame) == 1 {
		g.message(msgDemoGame)
		if g.yesNo(0x82C) != 'Y' {
			return
		}
	}
	g.sb(vJoySaved, g.b(vJoystick))
	g.sb(vJoystick, 0)
	g.sw(vDemoFile, g.openFile(fnDemo))
	g.restartDemo()
}

// restartDemo — L_15e2: перемотать запись, пропустить заголовок и начать
// игру одного игрока с засевом генератора 12345.
func (g *Game) restartDemo() {
	h := g.w(vDemoFile)
	g.seekFile(h, 0, 0)
	for {
		g.readFile(h, g.ds[vDemoKey:vDemoKey+2])
		if g.b(vDemoKey) == 0x0D {
			break
		}
	}
	g.sw(vSeed, 0x3039)
	g.sb(vSeedUses, 0)
	g.sb(vDemo, 1)
	g.sw(vColSel+6, 0x140)
	g.sw(vColDescr+6, 0)
	g.applySettings()
	g.sw(selGame, 0x140)
	g.sw(descrGame, 0)
	g.menuEnter(4)
}

// demoKey — L_16b5: во время демонстрации вместо клавиатуры читается запись.
// Нажатие настоящей клавиши прекращает демонстрацию.
func (g *Game) demoKey(ax uint16) uint16 {
	if ax != noKey {
		g.demoClose()
		g.sw(vDots, 0)
		g.sb(vStop, 1)
		g.sb(vDemoStop, 1)
		return noKey
	}
	h := g.w(vDemoFile)
	if g.w(vDemoCount) == 0 {
		g.readFile(h, g.ds[vDemoKey:vDemoKey+2])
		ax = g.w(vDemoKey)
		if ax == noKey {
			g.readFile(h, g.ds[vDemoCount:vDemoCount+2])
			g.decw(vDemoCount)
			ax = noKey
		}
	} else {
		g.decw(vDemoCount)
		ax = noKey
	}
	if byte(ax) == asciiEsc {
		return g.demoEnded()
	}
	return ax
}

// demoEnded — L_1634: запись кончилась — заставка; если за круг заставки
// клавишу не нажали, демонстрация начинается заново.
func (g *Game) demoEnded() uint16 {
	g.sb(vDemo, 0)
	g.clearPage0()
	g.sb(vDemoTitle, 1)
	ax := g.titleLoop()
	g.sb(vDemoTitle, 0)
	if ax == noKey {
		g.restartDemo()
	}
	g.demoClose()
	g.sw(vDots, 0)
	g.sb(vStop, 1)
	g.sb(vDemoStop, 1)
	return noKey
}

// demoClose — L_1697: закрыть запись и вернуть управление джойстиком.
func (g *Game) demoClose() {
	g.sb(vDemo, 0)
	g.closeFile(g.w(vDemoFile))
	g.sw(0x1398, 0)
	g.sb(vJoystick, g.b(vJoySaved))
}

// yesNo — L_1761: окно Yes/No в позиции di. Возвращает «Y», «N» или «E» (Esc).
func (g *Game) yesNo(di uint16) byte {
	g.sw(vStep, di)
	si := uint16(0)
	for r := 0; r < 4; r++ {
		for c := 0; c < 7; c++ {
			g.sw(vYesNoSave+si, g.v.TextRW(di))
			si += 2
			di += 2
		}
		di += 0x92
	}
	di = g.w(vStep)
	si = 0
	for r := 0; r < 4; r++ {
		for c := 0; c < 7; c++ {
			g.v.TextW(di, 0x0400|uint16(g.b(vYesNoBox+si)))
			si++
			di += 2
		}
		di += 0x92
	}
	di = g.w(vStep)
	for _, o := range []uint16{0xA5, 0xA7, 0xA9} {
		g.v.TextB(di+o, 1)
	}
	swap := func() {
		di := g.w(vStep)
		for i := 0; i < 3; i++ {
			a, b := g.v.TextRB(di+0xA5), g.v.TextRB(di+0x145)
			g.v.TextB(di+0xA5, b)
			g.v.TextB(di+0x145, a)
			di += 2
		}
	}
loop:
	for {
		ax := g.getKey()
		al, ah := byte(ax), byte(ax>>8)
		yes, no := false, false
		switch {
		case al == 0:
			yes, no = ah == scanUp, ah == scanDown
		case al == asciiEsc:
			g.sb(vYesNo, 'E')
			break loop
		case al == asciiEnter:
			break loop
		case al == 'Y' || al == 'y':
			yes = true
		case al == 'N' || al == 'n':
			no = true
		}
		if yes && g.b(vYesNo) != 'Y' {
			g.sb(vYesNo, 'Y')
			swap()
		}
		if no && g.b(vYesNo) != 'N' {
			g.sb(vYesNo, 'N')
			swap()
		}
	}
	al := g.b(vYesNo)
	g.sb(vYesNo, 'Y')
	di = g.w(vStep)
	si = 0
	for r := 0; r < 4; r++ {
		for c := 0; c < 7; c++ {
			g.v.TextW(di, g.w(vYesNoSave+si))
			si += 2
			di += 2
		}
		di += 0x92
	}
	return al
}

// controlMenu — L_18db: окно выбора управления (клавиатура или джойстик).
func (g *Game) controlMenu() {
	g.sb(vControlOld, g.b(vControl))
	di := g.w(vControlPos)
	si := uint16(0)
	for r := 0; r < 7; r++ {
		for c := 0; c < 14; c++ {
			g.sw(vControlSave+si, g.v.TextRW(di))
			si += 2
			di += 2
		}
		di += 0x84
	}
	si = 0
	di = g.w(vControlPos)
	ah := byte(1)
	for r := 0; r < 7; r++ {
		for c := 0; c < 14; c++ {
			al := g.b(vControlBox + si)
			ah = 4
			if al <= 0x7B {
				ah = 1
			}
			g.v.TextW(di, uint16(ah)<<8|uint16(al))
			si++
			di += 2
		}
		di += 0x84
	}
	di = g.w(vControlPos) + 0x1E7
	if g.b(vControl) != 'J' {
		di += 0x140
	}
	for i := 0; i < 8; i++ {
		g.v.TextB(di, ah)
		di += 2
	}
	g.message(msgKeyboard)
	swap := func() {
		di := g.w(vControlPos)
		for i := 0; i < 8; i++ {
			a, b := g.v.TextRB(di+0x1E7), g.v.TextRB(di+0x327)
			g.v.TextB(di+0x1E7, b)
			g.v.TextB(di+0x327, a)
			di += 2
		}
	}
	var al byte
loop:
	for {
		ax := g.getKey()
		k, sc := byte(ax), byte(ax>>8)
		kbd, joy := false, false
		switch {
		case k == 0:
			kbd, joy = sc == scanUp, sc == scanDown
		case k == asciiEsc:
			g.sb(vControl, g.b(vControlOld))
			al = 'E'
			break loop
		case k == asciiEnter:
			al = g.b(vControl)
			break loop
		case k == 'K' || k == 'k':
			kbd = true
		case k == 'J' || k == 'j':
			joy = true
		}
		if kbd && g.b(vControl) != 'K' {
			g.sb(vControl, 'K')
			g.message(msgKeyboard)
			swap()
		}
		if joy && g.b(vControl) != 'J' {
			g.sb(vControl, 'J')
			g.message(msgJoystick)
			swap()
		}
	}
	g.applyControl(al)
	si = 0
	di = g.w(vControlPos)
	for r := 0; r < 7; r++ {
		for c := 0; c < 14; c++ {
			g.v.TextW(di, g.w(vControlSave+si))
			si += 2
			di += 2
		}
		di += 0x84
	}
}

// applyControl — L_185c: включить выбранное управление; для джойстика —
// проверить его наличие и замерить центр.
func (g *Game) applyControl(al byte) {
	switch al {
	case 'E':
		return
	case 'K':
		g.sb(vJoystick, 0)
		g.sb(vControl, 'K')
		return
	}
	g.spin(10000 * 3)
	if g.joyPort() == 0xFF {
		g.message(msgNoJoy)
		if g.yesNo(0xAF4) != 'Y' {
			g.sb(vJoystick, 0)
			g.sb(vControl, 'K')
			return
		}
	}
	g.message(msgCenterJoy)
	for {
		ax := g.getKey()
		if byte(ax) == asciiEsc {
			g.sb(vJoystick, 0)
			g.sb(vControl, 'K')
			return
		}
		if byte(ax) == asciiEnter {
			break
		}
	}
	g.sw(vJoyCX, g.joyAxis(1))
	g.sw(vJoyCY, g.joyAxis(2))
	g.sb(vJoystick, 1)
}

// joyPort — чтение порта игрового адаптера 201h: без джойстика все биты единичные.
func (g *Game) joyPort() byte {
	return 0xFF
}

// joyAxis — L_18c8: время разряда оси (бит mask порта 201h) в итерациях
// цикла. Без джойстика цикл доходит до предела, и результат равен 0.
func (g *Game) joyAxis(mask byte) uint16 {
	g.spin(0x400*2 + 0x10000*3)
	return 0
}

// escMenu — L_0c13: меню по Esc во время игры. Байты страницы 0, которые
// портит текстовый режим (0..1FFFh и 4000h..6007h), сохраняются в буфер и
// потом возвращаются; «Position» показывает игру до нажатия клавиши.
func (g *Game) escMenu() {
	g.sb(vInMenu, 1)
	g.gameTime()
	g.speakerOff()
	for {
		g.saveScreen()
		g.addw(segBuf, 0x0E30)
		g.textMode(true)
		g.setTextFonts()
		if g.b(vAfterGame) == 1 {
			g.clearText()
			g.afterGame()
		} else {
			g.mainMenu()
			if g.b(vStop) == 1 {
				break
			}
		}
		// L_0cb6
		g.graphMode(true)
		g.setPalette(palBlack)
		g.sw(segBuf, g.w(segBuf)-0x0E30)
		g.restoreScreen()
		if g.b(vEnding) != 1 {
			g.setPalette(palImage)
		}
		g.sb(vEnding, 0)
		g.v.OutGC(0x0105)
		if g.b(vPosition) != 1 {
			break
		}
		g.getKey()
	}
	g.gameTime()
	g.sb(vInMenu, 0)
	g.sb(vAfterGame, 0)
}

// saveScreen — L_0c2a: сохранить плоскости 3..0 байтов 0..1FFFh и 4000h..6007h
// страницы 0 в буфер (по 4 байта на смещение, адрес в буфере 16-битный).
func (g *Game) saveScreen() {
	base := g.bufBase()
	v := g.v
	di := uint16(0)
	save := func(si uint16, n int) {
		for ; n > 0; n-- {
			for p := 0; p < 4; p++ {
				v.OutGC(uint16(3-p)<<8 | 4)
				g.buf[base+int(di+uint16(p))] = v.Read(si)
			}
			si++
			di += 4
		}
	}
	save(0, 0x2000)
	save(0x4000, 0x2008)
	g.spin(0x4008 * 20)
}

// restoreScreen — L_0cd4: вернуть сохранённые байты страницы 0.
func (g *Game) restoreScreen() {
	base := g.bufBase()
	v := g.v
	v.OutGC(0x0005)
	v.OutGC(0xFF08)
	v.OutGC(0x0F00)
	si := uint16(0)
	b := func(i uint16) byte { return g.buf[base+int(i)] }
	put := func(di uint16, n int) {
		for ; n > 0; n-- {
			v.OutGC(0x0003)
			v.OutGC(0x0F01)
			v.Write(di, 0)
			v.OutGC(0x0803)
			v.OutGC(0x0E01)
			v.Read(di)
			v.Write(di, b(si+3))
			v.OutGC(0x0D01)
			v.Read(di)
			v.Write(di, b(si+2))
			v.OutGC(0x0B01)
			v.Read(di)
			v.Write(di, b(si+1))
			v.OutGC(0x0701)
			v.Read(di)
			v.Write(di, b(si))
			si += 4
			di++
		}
	}
	put(0, 0x2000)
	put(0x4000, 0x2008)
	g.spin(0x4008 * 30)
}
