package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"os"
	"testing"
	"time"

	"github.com/therox/go-cdman/internal/ega"
	"github.com/therox/go-cdman/internal/game"
	"github.com/therox/go-cdman/internal/host"
	"github.com/therox/go-cdman/internal/sfx"
)

// testFiles находит файлы игры: каталог из CDMAN_DATA или Source/.
// Без них тест пропускается: файлы оригинала в проект не входят.
func testFiles(t *testing.T) *host.Files {
	t.Helper()
	dir := os.Getenv("CDMAN_DATA")
	if dir == "" {
		dir = "Source"
	}
	if err := host.Check(dir); err != nil {
		t.Skip("нет файлов игры (задайте CDMAN_DATA):", err)
	}
	return &host.Files{Dir: dir}
}

// Состояние на тиках демонстрации, снятое с оригинала в эмуляторе:
// SHA-256 байтов DS:0000–0271 (без счётчиков игрового времени, которые
// зависят от часов) и CRC-32 плоскостей видимой страницы.
var demoReference = map[int]struct{ state, vram string }{
	1000: {"1019e4fdd63ffc0300a4fd46ffb352c176dac360fbf48de9b4ee73e50f6f7590", "8fcea938"},
	1795: {"dbe42c1cb37e301864c05e5dff4300cc34a6f8026941f631dcd660184daac076", "2521f9fe"},
}

// timeBytes — переменные, зависящие от часов (учёт игрового времени).
var timeBytes = []int{0x252, 0x253, 0x254, 0x255, 0x256, 0x25F, 0x260, 0x261, 0x268, 0x269, 0x26A}

// Демонстрация (запись DEMO.CDM) играется так же, как в оригинале:
// состояние игры и картинка совпадают с эталоном.
func TestDemoMatchesOriginal(t *testing.T) {
	files := testFiles(t)
	ds, err := files.LoadDS()
	if err != nil {
		t.Fatal(err)
	}
	v := ega.New(ega.ROMFont)
	kb := &host.Keyboard{}
	press := func(name string) func() {
		return func() {
			k, ok := host.KeyByName[name]
			if !ok {
				k = host.CharKey(name[0])
			}
			kb.Press(k.Scan, k.ASCII)
		}
	}
	clk := host.NewScriptClock([]host.ScriptEvent{
		{At: 26, Do: press("enter")},
		{At: 28, Do: press("left")},
		{At: 28.3, Do: press("left")},
		{At: 28.6, Do: press("d")},
		{At: 29, Do: press("enter")},
		{At: 600, Do: func() { panic(errQuit) }},
	})
	mf := &memFiles{Files: files, written: map[string][]byte{}}
	g := game.New(v, clk, kb, sfx.NewOfflineMixer(), mf, ds, time.Date(1992, 1, 1, 0, 0, 0, 0, time.Local))
	got := map[int][2]string{}
	tick := 0
	g.OnTick(func(g *game.Game) {
		if _, ok := demoReference[tick]; ok {
			p := append([]byte(nil), g.DS()[:0x272]...)
			for _, a := range timeBytes {
				p[a] = 0
			}
			sum := sha256.Sum256(p)
			pl := crc32.NewIEEE()
			buf := make([]byte, ega.PageSize)
			for k := 0; k < 4; k++ {
				for i := range buf {
					buf[i] = v.Plane(k, uint16(i))
				}
				pl.Write(buf)
			}
			got[tick] = [2]string{hex.EncodeToString(sum[:]), hex.EncodeToString(pl.Sum(nil))}
		}
		tick++
		if tick > 1795 {
			panic(errQuit)
		}
	})
	func() {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok && errors.Is(e, errQuit) {
					return
				}
				panic(r)
			}
		}()
		_ = g.Run()
	}()
	for n, want := range demoReference {
		g, ok := got[n]
		if !ok {
			t.Fatalf("демонстрация не дошла до тика %d (всего %d)", n, tick)
		}
		if g[0] != want.state {
			t.Errorf("тик %d: состояние %s, ожидалось %s", n, g[0], want.state)
		}
		if g[1] != want.vram {
			t.Errorf("тик %d: картинка %s, ожидалось %s", n, g[1], want.vram)
		}
	}
}

// Сценарий автономного прогона разбирается и выполняется.
func TestScript(t *testing.T) {
	files := testFiles(t)
	dir := t.TempDir()
	script := dir + "/s.txt"
	err := os.WriteFile(script, []byte("# заставка, меню и демонстрация\n12 shot title.png\n26 key enter\n27 shot menu.png\n28 quit\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if err := runScript(files, script, dir, "", 1, debugOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"title.png", "menu.png"} {
		if fi, err := os.Stat(dir + "/" + n); err != nil || fi.Size() == 0 {
			t.Errorf("нет снимка %s: %v", n, err)
		}
	}
}
