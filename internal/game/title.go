package game

// Переменные заставки.
const (
	vTitleDelay = 0x3457 // задержка шага анимации (g_106a)
	vCheck      = 0x3459 // самопроверка скорости: «A» или «B»
	vCheckLeft  = 0x345A // сколько замеров осталось
	vManSrc1    = 0x345B // кадры двух бегающих CD-Man (страница 1)
	vManSrc2    = 0x345D
	vManDst1    = 0x345F // их места на странице 0
	vManDst2    = 0x3461
	vManStep1   = 0x3463 // смещение за шаг (±1 строка)
	vManStep2   = 0x3465
	vManCount   = 0x3467 // шагов до разворота
	vManFrame   = 0x3469 // номер кадра
	vBlobPos    = 0x346B // места десяти синих облаков
	vSparkX     = 0x3481 // блики: x
	vSparkY     = 0x34B5 // y
	vSparkState = 0x34E9 // состояние (0..40h, шаг 2)
	vSparkIdx   = 0x347F // номер текущего блика × 2
	vSparkColor = 0x355F
	vJitterMask = 0x0499 // растворение: маска и смещение дрожания
	vJitterBias = 0x049B
	vJitterSh   = 0x049D // сдвиг за проход (зависит от скорости машины)
	vPixelMask  = 0x1031
	vTmp        = 0x0296 // временная ячейка оригинала
)

// title — L_4bf0: заставка, если клавиша ещё не нажата.
func (g *Game) title() uint16 {
	if ax := g.poll(); ax != noKey {
		return ax
	}
	return g.titleLoop()
}

// titleLoop — L_4bf9: заставка до нажатия клавиши. Между демонстрациями
// (g_1397=1) показывается один круг анимации; тогда 0FFFFh означает, что
// клавиша так и не была нажата.
func (g *Game) titleLoop() uint16 {
	g.sw(vDelayMul, g.w(vTitleDelay))
	// заставка идёт с калиброванной задержкой, без поправки скорости из меню
	saved := g.w(vDelaySaved)
	g.sw(vDelaySaved, g.w(vDelay))
	g.sw(vDelay, saved)
	g.loadFile(fnTitle)
	g.decodeImage(0xA6D6)
	g.setPalette(palBlack)
	g.clearPage0()
	g.titleStatic()
	g.setPalette(palImage)
	g.sw(vManSrc1, 0x6D60)
	g.sw(vManSrc2, 0x9240)
	g.sw(vManDst1, 0x2D5D)
	g.sw(vManDst2, 0x2D89)
	g.sw(vManCount, 0x91)
	g.sw(vManFrame, 0xFFFF)
	g.sw(vManStep1, 0x50)
	g.sw(vManStep2, 0xFFB0)
	g.titleStep()
	ax := g.dissolve(0xB720, 0x2A9E)
	for ax == noKey {
		g.sparkles()
		g.titleStep()
		if ax = g.poll(); ax != noKey {
			break
		}
		if g.b(vDemoTitle) == 1 && g.w(vManCount) == 0x91 {
			ax = noKey
			break
		}
	}
	g.sw(vTitleDelay, g.w(vDelayMul))
	saved = g.w(vDelaySaved)
	g.sw(vDelaySaved, g.w(vDelay))
	g.sw(vDelay, saved)
	return ax
}

// titleStatic — L_4cb0: неподвижные части заставки со страницы 1.
func (g *Game) titleStatic() {
	// L_4d57: подписи авторов
	g.v.CopyBlock(0xD062, 0x6901, 0x0B, 0x0D)
	g.v.CopyBlock(0xD602, 0x6948, 0x07, 0x0D)
	// логотип «CD-MAN Version 2.0»
	g.v.CopyBlock(0xBAA4, 0x19B7, 0x22, 0x33)
	// десять синих облаков
	for bx := uint16(0); bx < 20; bx += 2 {
		g.v.CopyBlock(0xBAD0, g.w(vBlobPos+bx), 0x0C, 0x42)
	}
	// три полосы травы со страницы 1
	si, di := uint16(0xD200), uint16(0x46B8)
	for i := 0; i < 3; i++ {
		g.v.CopyBlock(si, di, 0x20, 9)
		si += 9 * 80
		di += 9*80 + 0xF00
	}
	di = 0x4988
	for i := 0; i < 2; i++ {
		g.v.CopyBlock(0xCBE0, di, 0x20, 0x30)
		di += 0x30*80 + 0x2D0
	}
	g.spin(0x33*0x22*12 + 10*0x42*0x0C*12 + 3*9*0x20*12 + 2*0x30*0x20*12)
}

// titleStep — L_4fa5: шаг двух бегающих CD-Man.
func (g *Game) titleStep() {
	g.delayN()
	g.v.CopyBlock(g.w(vManSrc1), g.w(vManDst1), 10, 0x3B)
	g.v.CopyBlock(g.w(vManSrc2), g.w(vManDst2), 10, 0x3B)
	g.spin(2 * 0x3B * 56)
	g.incw(vManCount)
	g.incw(vManFrame)
	if g.w(vManCount) == 0x123 {
		g.sw(vManCount, 0)
		g.sw(vManStep1, -g.w(vManStep1))
		g.sw(vManStep2, -g.w(vManStep2))
	}
	g.addw(vManDst1, g.w(vManStep1))
	g.addw(vManDst2, g.w(vManStep2))
	switch ax := g.w(vManFrame); {
	case ax == 0x1F:
		g.sw(vManFrame, 0xFFFF)
		g.sw(vManSrc1, 0x6D60)
		g.addw(vManSrc2, 0x122A)
		g.speedCheck()
	case ax == 0x0F:
		g.addw(vManSrc1, 0x122A)
		g.sw(vManSrc2, 0x6D60)
	case ax&7 == 7:
		g.addw(vManSrc1, 0x122A)
		g.addw(vManSrc2, 0x122A)
	default:
		g.addw(vManSrc1, 0x0A)
		g.addw(vManSrc2, 0x0A)
	}
}

// speedCheck — L_501b: самопроверка скорости во время заставки (дважды
// замеряет круг анимации и сравнивает; ни на что не влияет).
func (g *Game) speedCheck() {
	switch g.b(vCheckLeft) {
	case 0:
		return
	case 2:
		bx := g.elapsed()
		g.sb(vCheck, 'A')
		if bx < 0x78 {
			g.sb(vCheck, 'B')
		}
	case 3:
	default:
		bx := g.elapsed()
		al := byte('A')
		if bx < 0x78 {
			al = 'B'
		}
		if al == g.b(vCheck) {
			_, _, dh, dl := g.dosTime()
			g.sw(vCalibStart, uint16(dh)<<8|uint16(dl))
			return
		}
		g.sb(vCheckLeft, 0)
		return
	}
	g.sb(vCheckLeft, g.b(vCheckLeft)-1)
	_, _, dh, dl := g.dosTime()
	g.sw(vCalibStart, uint16(dh)<<8|uint16(dl))
}

// dissolve — L_4442: «растворение» прямоугольника 160×87 точек со страницы 1
// (si) на страницу 0 (di): точки выводятся в случайном порядке за 8/g_049d
// проходов, источник дрожит всё меньше. Возвращает клавишу, прервавшую эффект.
func (g *Game) dissolve(si, di uint16) uint16 {
	v := g.v
	v.OutGC(0x0205)
	v.OutGC(0x0003)
	g.sw(vJitterMask, 0x01FE)
	g.sw(vJitterBias, 0x00FF)
	g.toPixels(si)
	g.shuffle()
	cl := g.b(vJitterSh) | 0x10
	for cl&1 == 0 {
		cl >>= 1
	}
	passes := int(cl >> 1)
	buf := g.buf[g.bufBase():]
	w16 := func(a int) uint16 { return uint16(buf[a]) | uint16(buf[a+1])<<8 }
	for ; passes > 0; passes-- {
		sh := g.b(vJitterSh)
		g.sw(vJitterMask, g.w(vJitterMask)>>sh&0xFFFE)
		g.sw(vJitterBias, g.w(vJitterBias)>>sh)
		bp := 0x3660
		for n := 0; n < 0x6CC; n++ {
			if ax := g.poll(); ax != noKey {
				return ax
			}
			dst := di + w16(bp)
			src := w16(bp + 2)
			bp += 4
			g.sb(vPixelMask, 0x80)
			for k := 0; k < 8; k++ {
				p := g.jitter(src)
				bl := byte(0x0F)
				if p <= 0x365F {
					bl = buf[p]
				}
				src++
				v.OutGC(uint16(g.b(vPixelMask))<<8 | 8)
				g.sb(vPixelMask, g.b(vPixelMask)>>1)
				v.Read(dst)
				v.Write(dst, bl)
			}
			g.spin(8*65 + 10)
		}
	}
	v.OutGC(0x0105)
	return noKey
}

// toPixels — L_44e8: переписать прямоугольник 20 байт × 87 строк видеопамяти
// в буфер по точке на байт (номер цвета в младших 4 битах).
func (g *Game) toPixels(si uint16) {
	v := g.v
	buf := g.buf[g.bufBase():]
	di := 0
	for r := 0; r < 0x57; r++ {
		for c := 0; c < 0x14; c++ {
			v.OutGC(0x0304)
			bh := v.Read(si)
			v.OutGC(0x0204)
			bl := v.Read(si)
			v.OutGC(0x0104)
			ah := v.Read(si)
			v.OutGC(0x0004)
			al := v.Read(si)
			for k := 0; k < 8; k++ {
				b := buf[di]
				b = b<<1 | bh>>7
				b = b<<1 | bl>>7
				b = b<<1 | ah>>7
				b = b<<1 | al>>7
				bh, bl, ah, al = bh<<1, bl<<1, ah<<1, al<<1
				buf[di] = b
				di++
			}
			si++
		}
		si += 0x3C
	}
	g.spin(0x57 * 0x14 * 80)
}

// shuffle — L_454a: случайный порядок 1740 байтовых позиций прямоугольника.
func (g *Game) shuffle() {
	buf := g.buf[g.bufBase():]
	w16 := func(a int) uint16 { return uint16(buf[a]) | uint16(buf[a+1])<<8 }
	s16 := func(a int, v uint16) { buf[a] = byte(v); buf[a+1] = byte(v >> 8) }
	bp, ax := uint16(0), uint16(0)
	di := 0
	for r := 0; r < 0x57; r++ {
		for c := 0; c < 0x14; c++ {
			s16(di+0x5190, bp)
			di += 2
			s16(di+0x5190, ax)
			di += 2
			ax += 8
			bp++
		}
		bp += 0x3C
	}
	last := 0x6CBC
	for di = 0x3660; di != 0x5190; {
		ax := g.random2()
		ax &= 0x07FF
		if ax > 0x6CB {
			ax -= 0x134
		}
		bx := int(ax) << 2
		for w16(bx+0x5190) == 0xFFFF {
			bx = (bx >> 1) &^ 3
		}
		s16(di, w16(bx+0x5190))
		s16(di+2, w16(bx+0x5192))
		di += 4
		s16(bx+0x5190, w16(last))
		s16(bx+0x5192, w16(last+2))
		s16(last, 0xFFFF)
		last -= 4
	}
	g.spin(0x6CC * 45)
}

// jitter — L_45d6: номер точки-источника со случайным смещением.
func (g *Game) jitter(si uint16) uint16 {
	bx, cx := g.w(vJitterMask), g.w(vJitterBias)
	ax := g.random2()
	si += ax&bx - cx
	g.sb(vTmp, byte(bx))
	bx = bx<<8 | bx>>8
	a := (ax & bx) >> 1
	si += a
	si += a >> 2
	g.sb(vTmp, byte(cx))
	cx = cx<<8 | cx>>8
	cx >>= 1
	si -= cx
	si -= cx >> 2
	return si
}

// sparkles — L_4d98: шаг 26 бликов на логотипе и облаках.
func (g *Game) sparkles() {
	g.v.OutGC(0x0205)
	for bx := uint16(0); bx <= 0x32; bx += 2 {
		g.sw(vSparkIdx, bx)
		st := g.w(vSparkState + bx)
		y, x := g.w(vSparkY+bx), g.w(vSparkX+bx)
		g.spark(st/2, y, x)
		st += 2
		if st > 0x40 {
			st = 0
		}
		g.sw(vSparkState+bx, st)
	}
	g.v.OutGC(0x0105)
	g.spin(26 * 60)
}

// spark — функции таблицы 351Dh (CS:4DDC–4F0D): зажечь блик радиуса r
// цветом (0, 9, 6) или стереть его, вернув точки фона.
func (g *Game) spark(state, y, x uint16) {
	put := func(r uint16, c byte) {
		g.sb(vSparkColor, c)
		if r == 0 {
			g.putPixel(y, x)
			return
		}
		g.putPixel(y+r, x)
		g.putPixel(y-r, x)
		g.putPixel(y, x+r)
		g.putPixel(y, x-r)
	}
	restore := func(r uint16) {
		if r == 0 {
			g.restorePixel(y, x)
			return
		}
		g.restorePixel(y+r, x)
		g.restorePixel(y-r, x)
		g.restorePixel(y, x+r)
		g.restorePixel(y, x-r)
	}
	switch state {
	case 0:
		put(0, 0)
	case 1:
		// L_4de5: верх, низ, лево, право
		g.sb(vSparkColor, 0)
		g.putPixel(y-1, x)
		g.putPixel(y+1, x)
		g.putPixel(y, x-1)
		g.putPixel(y, x+1)
	case 2:
		put(2, 9)
	case 3:
		put(3, 9)
	case 4:
		put(4, 6)
	case 5:
		put(5, 6)
	case 6:
		restore(5)
	case 7:
		restore(4)
	case 8:
		restore(3)
	case 9:
		restore(2)
	case 10:
		// L_4e88: верх, низ, лево, право
		g.restorePixel(y-1, x)
		g.restorePixel(y+1, x)
		g.restorePixel(y, x-1)
		g.restorePixel(y, x+1)
	case 11:
		restore(0)
	}
}

// putPixel — L_4f0e: точка цветом g_355f (режим записи 2).
func (g *Game) putPixel(y, x uint16) {
	bx := y*80 + x>>3
	cl := byte(x&7) ^ 7
	g.v.OutGC(uint16(1)<<(8+cl) | 8)
	g.v.OutGC(0x0003)
	g.v.Read(bx)
	g.v.Write(bx, g.b(vSparkColor))
}

// restorePixel — L_4f3f: вернуть точку фона из копии на странице 1.
func (g *Game) restorePixel(y, x uint16) {
	bx := y*80 + x>>3
	cl := byte(x&7) ^ 7
	si := bx + 0xA0ED
	if g.w(vSparkIdx) > 0x24 {
		si += 0xEB95
	}
	ch := byte(1) << cl
	var bl byte
	for ah := 3; ah >= 0; ah-- {
		g.v.OutGC(uint16(ah)<<8 | 4)
		bh := g.v.Read(si) & ch
		bl <<= 1
		if bh != 0 {
			bl |= 1
		}
	}
	g.sb(vSparkColor, bl)
	g.v.OutGC(uint16(ch)<<8 | 8)
	g.v.OutGC(0x0003)
	g.v.Read(bx)
	g.v.Write(bx, bl)
}
