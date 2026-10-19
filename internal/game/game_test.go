package game

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/therox/go-cdman/internal/dat"
	"github.com/therox/go-cdman/internal/ega"
	"github.com/therox/go-cdman/internal/host"
)

type nullSound struct{}

func (nullSound) Sync(float64)            {}
func (nullSound) Divisor(float64, uint16) {}

// newTestGame создаёт игру с сегментом данных из CDMAN2.EXE.
func newTestGame(t *testing.T) *Game {
	t.Helper()
	dir := os.Getenv("CDMAN_DATA")
	if dir == "" {
		dir = "../../Source"
	}
	raw, err := os.ReadFile(filepath.Join(dir, "CDMAN2.EXE"))
	if err != nil {
		t.Skip("нет файлов игры (задайте CDMAN_DATA):", err)
	}
	ds, err := dat.ParseEXE(raw)
	if err != nil {
		t.Fatal(err)
	}
	files := &host.Files{Dir: dir}
	return New(ega.New(ega.ROMFont), host.NewScriptClock(nil), &host.Keyboard{}, nullSound{}, files, ds,
		time.Date(1992, 1, 1, 0, 0, 0, 0, time.UTC))
}

// Генератор L_4a97: seed·41, пересев каждые 100 вызовов (во время демонстрации — 12345).
func TestRandom(t *testing.T) {
	g := &Game{ds: make([]byte, 0x10000), clk: host.NewScriptClock(nil), speed: 1}
	g.sb(vDemo, 1)
	g.sw(vSeed, 0x3039)
	s := uint16(0x3039)
	for i := 1; i < 100; i++ {
		s *= 41
		if v := g.random(); v != s {
			t.Fatalf("вызов %d: %04x, ожидалось %04x", i, v, s)
		}
	}
	// сотый вызов засевает генератор заново
	if v := g.random(); v != uint16(0x3039*41&0xFFFF) {
		t.Fatalf("после пересева %04x", v)
	}
}

// На эталонной машине калибровка даёт задержку 1424 и шаг 22.
func TestCalibrate(t *testing.T) {
	g := newTestGame(t)
	g.calibrate()
	if d, s := g.w(vDelay), g.w(vDelayStep); d != 1424 || s != 22 {
		t.Fatalf("задержка %d, шаг %d", d, s)
	}
	if g.b(0x049D) != 1 {
		t.Fatalf("сдвиг растворения %d", g.b(0x049D))
	}
}

// Карта клеток первого экрана: число точек и выход из клетки монстров.
func TestInitMaze(t *testing.T) {
	g := newTestGame(t)
	g.initMaze(0)
	dots := 0
	for c := uint16(0x16); c < 0x16+11*21; c++ {
		if g.b(vMaze+c)&8 != 0 {
			dots++
		}
	}
	if int(g.w(vDots)) != dots || dots < 100 {
		t.Fatalf("точек %d, счётчик %d", dots, g.w(vDots))
	}
	if g.b(vMaze+0x73)&0x40 == 0 {
		t.Fatal("из клетки 73h должен быть выход вниз")
	}
}

// Время между замерами считается с той же погрешностью, что в оригинале.
func TestElapsed(t *testing.T) {
	g := newTestGame(t)
	cases := []struct {
		start, now [2]byte // секунды, сотые
		want       uint16
	}{
		{[2]byte{10, 20}, [2]byte{10, 84}, 64},
		{[2]byte{10, 90}, [2]byte{11, 10}, 19},
		{[2]byte{59, 50}, [2]byte{0, 50}, 99},
	}
	for _, c := range cases {
		g.sw(vCalibStart, uint16(c.start[0])<<8|uint16(c.start[1]))
		g.timeShift = float64(c.now[0]) + float64(c.now[1])/100 + 0.001 - g.clk.Now()
		if got := g.elapsed(); got != c.want {
			t.Errorf("%v → %v: %d, ожидалось %d", c.start, c.now, got, c.want)
		}
	}
}
