package ega

import "testing"

// Байт копируется через защёлки во всех четырёх плоскостях (режим записи 1).
func TestLatchCopy(t *testing.T) {
	c := New(ROMFont)
	c.SetMode(0x10)
	for p := 0; p < 4; p++ {
		c.planes[p][100] = byte(0x11 * (p + 1))
	}
	c.OutGC(0x0105)
	c.Copy(100, 200)
	for p := 0; p < 4; p++ {
		if got := c.Plane(p, 200); got != byte(0x11*(p+1)) {
			t.Fatalf("плоскость %d: %02x", p, got)
		}
	}
}

// Распаковка картинок: set/reset для трёх плоскостей и OR с защёлками для четвёртой.
func TestSetResetOr(t *testing.T) {
	c := New(ROMFont)
	c.SetMode(0x10)
	for p := 0; p < 4; p++ {
		c.planes[p][5] = 0x0F
	}
	c.OutGC(0x1003) // OR
	c.OutGC(0x0E01) // set/reset (0) для плоскостей 1–3
	c.Read(5)
	c.Write(5, 0xF0)
	if got := c.Plane(0, 5); got != 0xFF {
		t.Fatalf("плоскость 0: %02x", got)
	}
	for p := 1; p < 4; p++ {
		if got := c.Plane(p, 5); got != 0x0F {
			t.Fatalf("плоскость %d изменилась: %02x", p, got)
		}
	}
}

// Режим записи 2 с битовой маской ставит одну точку цвета из данных.
func TestWriteMode2(t *testing.T) {
	c := New(ROMFont)
	c.SetMode(0x10)
	c.OutGC(0x0205)
	c.OutGC(0x2008) // точка 2 в байте
	c.Read(0)
	c.Write(0, 0x0B)
	var f Frame
	c.Capture(&f, false)
	if f.Pix[2] != 0x0B || f.Pix[1] != 0 || f.Pix[3] != 0 {
		t.Fatalf("точки %v", f.Pix[:8])
	}
}

// Символы с ярким цветом берутся из второго набора, остальные — из шрифта ПЗУ.
func TestTextFonts(t *testing.T) {
	c := New(ROMFont)
	user := make([]byte, 14)
	for i := range user {
		user[i] = 0xFF
	}
	c.LoadFont(1, user, 'A', 1, 14)
	c.SelectFonts(4)
	c.SetBlink(false)
	c.TextW(0, 0x0141) // 'A' цветом 1
	c.TextW(2, 0x0941) // 'A' цветом 9
	var f Frame
	c.Capture(&f, false)
	for x := 8; x < 16; x++ {
		if f.Pix[x] != 9 {
			t.Fatalf("второй символ должен быть залит: %v", f.Pix[8:16])
		}
	}
	lit := 0
	for y := 0; y < FontRows; y++ {
		for x := 0; x < 8; x++ {
			if f.Pix[y*W+x] == 1 {
				lit++
			}
		}
	}
	if lit == 0 || lit == 8*FontRows {
		t.Fatalf("первый символ должен быть из шрифта ПЗУ: %d точек", lit)
	}
}
