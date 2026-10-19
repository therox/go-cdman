package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/therox/go-cdman/internal/ega"
	"github.com/therox/go-cdman/internal/game"
	"github.com/therox/go-cdman/internal/host"
	"github.com/therox/go-cdman/internal/sfx"
)

var errQuit = errors.New("quit")

// memFiles — файлы игры для автономного прогона: изменяемые файлы
// (HIGHSC.CDM) остаются в памяти и на диск не пишутся.
type memFiles struct {
	*host.Files
	written map[string][]byte
}

func (m *memFiles) Read(name string) ([]byte, error) {
	if b, ok := m.written[strings.ToUpper(name)]; ok {
		return b, nil
	}
	return m.Files.Read(name)
}

func (m *memFiles) Write(name string, data []byte) error {
	m.written[strings.ToUpper(name)] = append([]byte(nil), data...)
	return nil
}

// runScript выполняет игру без окна в виртуальном времени по сценарию.
//
// Строки сценария: «<время, с> <команда> [аргументы]», команды:
// key <имя|символ> — нажатие; shot <файл> — снимок экрана;
// tickkey <номер тика> <клавиша> — нажатие в начале тика; quit.
// Время суток в начале прогона — полночь 1 января 1992 года, как у эмулятора.
// debugOptions — отладочные параметры автономного прогона.
type debugOptions struct {
	fixedSeed bool
	screen    int
	stateLog  string
}

func runScript(files *host.Files, path, outDir, wavPath string, speed float64, dbg debugOptions) error {
	v := ega.New(ega.ROMFont)
	kb := &host.Keyboard{}
	mixer := sfx.NewOfflineMixer()
	ds, err := files.LoadDS()
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var events []host.ScriptEvent
	tickKeys := map[int]host.Key{}
	callKeys := map[uint16]host.Key{}
	var levelPoke [][2]uint16
	var g *game.Game
	num := func(s string) (uint16, error) {
		v, err := strconv.ParseUint(s, 0, 16)
		return uint16(v), err
	}
	parseKey := func(s string) (host.Key, bool) {
		if k, ok := host.KeyByName[s]; ok {
			return k, true
		}
		if len(s) == 1 {
			return host.CharKey(s[0]), true
		}
		return host.Key{}, false
	}
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if i := strings.IndexByte(text, '#'); i >= 0 {
			text = text[:i]
		}
		fs := strings.Fields(text)
		if len(fs) == 0 {
			continue
		}
		if len(fs) < 2 {
			return fmt.Errorf("%s:%d: ожидается «время команда»", path, line)
		}
		at, err := strconv.ParseFloat(fs[0], 64)
		if err != nil {
			return fmt.Errorf("%s:%d: %w", path, line, err)
		}
		args := fs[2:]
		var do func()
		switch fs[1] {
		case "key":
			if len(args) != 1 {
				return fmt.Errorf("%s:%d: key <имя>", path, line)
			}
			k, ok := parseKey(args[0])
			if !ok {
				return fmt.Errorf("%s:%d: неизвестная клавиша %q", path, line, args[0])
			}
			do = func() { kb.Press(k.Scan, k.ASCII) }
		case "shot":
			if len(args) != 1 {
				return fmt.Errorf("%s:%d: shot <файл>", path, line)
			}
			name := args[0]
			do = func() {
				out, err := os.Create(filepath.Join(outDir, name))
				if err != nil {
					panic(err)
				}
				defer out.Close()
				if err := png.Encode(out, v.Snapshot()); err != nil {
					panic(err)
				}
			}
		case "quit":
			do = func() { panic(errQuit) }
		case "poke":
			if len(args) != 2 {
				return fmt.Errorf("%s:%d: poke <адрес> <значение>", path, line)
			}
			a, err1 := num(args[0])
			v, err2 := num(args[1])
			if err1 != nil || err2 != nil {
				return fmt.Errorf("%s:%d: poke <адрес> <значение>", path, line)
			}
			do = func() { g.Poke(a, v) }
		case "levelpoke":
			if len(args) != 2 {
				return fmt.Errorf("%s:%d: levelpoke <адрес> <значение>", path, line)
			}
			a, err1 := num(args[0])
			v, err2 := num(args[1])
			if err1 != nil || err2 != nil {
				return fmt.Errorf("%s:%d: levelpoke <адрес> <значение>", path, line)
			}
			levelPoke = append(levelPoke, [2]uint16{a, v})
			continue
		case "blinkkey", "keyhook":
			// blinkkey <клавиша> — то же, что keyhook 0992 <клавиша>
			if fs[1] == "blinkkey" {
				args = append([]string{"0x0992"}, args...)
			}
			if len(args) != 2 {
				return fmt.Errorf("%s:%d: keyhook <адрес> <клавиша>", path, line)
			}
			a, err := num(args[0])
			k, ok := parseKey(args[1])
			if err != nil || !ok {
				return fmt.Errorf("%s:%d: keyhook <адрес> <клавиша>", path, line)
			}
			callKeys[a] = k
			continue
		case "tickkey":
			if len(args) != 2 {
				return fmt.Errorf("%s:%d: tickkey <тик> <клавиша>", path, line)
			}
			n, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("%s:%d: %w", path, line, err)
			}
			k, ok := parseKey(args[1])
			if !ok {
				return fmt.Errorf("%s:%d: неизвестная клавиша %q", path, line, args[1])
			}
			tickKeys[n] = k
			continue
		default:
			return fmt.Errorf("%s:%d: неизвестная команда %q", path, line, fs[1])
		}
		events = append(events, host.ScriptEvent{At: at, Do: do})
	}
	if err := sc.Err(); err != nil {
		return err
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].At < events[j].At })
	events = append(events, host.ScriptEvent{At: 3600, Do: func() { panic(errQuit) }})

	clk := host.NewScriptClock(events)
	start := time.Date(1992, 1, 1, 0, 0, 0, 0, time.Local)
	mf := &memFiles{Files: files, written: map[string][]byte{}}
	g = game.New(v, clk, kb, mixer, mf, ds, start)
	if levelPoke != nil {
		g.OnLevel(func() {
			for _, p := range levelPoke {
				g.Poke(p[0], p[1])
			}
		})
	}
	g.SetSpeed(speed)
	g.Debug(dbg.fixedSeed, dbg.screen)
	var lw *bufio.Writer
	if dbg.stateLog != "" {
		lf, err := os.Create(dbg.stateLog)
		if err != nil {
			return err
		}
		defer lf.Close()
		lw = bufio.NewWriter(lf)
		defer lw.Flush()
	}
	ticks := 0
	if len(callKeys) > 0 {
		g.OnCall(func(addr uint16) {
			if k, ok := callKeys[addr]; ok {
				kb.Press(k.Scan, k.ASCII)
			}
		})
	}
	g.OnTick(func(g *game.Game) {
		if lw != nil {
			fmt.Fprintf(lw, "S %s\n", stateLine(g, v, clk.Now()))
		}
		if k, ok := tickKeys[ticks]; ok {
			kb.Press(k.Scan, k.ASCII)
		}
		ticks++
	})
	err = func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok && errors.Is(e, errQuit) {
					return
				}
				err = fmt.Errorf("сбой игры: %v", r)
			}
		}()
		return g.Run()
	}()
	if err != nil {
		return err
	}
	if wavPath != "" {
		buf := make([]float32, int((clk.Now()+0.5)*sfx.Rate))
		mixer.Render(buf)
		return writeWAV(wavPath, buf)
	}
	return nil
}

// stateLine — состояние игры в том же формате, что и дамп из эмулятора
// оригинала: состояние игрока DS:0..271h, избранные переменные, CRC всего
// сегмента данных и видимой страницы.
func stateLine(g *game.Game, v *ega.Card, now float64) string {
	ds := g.DS()[:0x3560]
	h := func(a, b int) string { return hex.EncodeToString(ds[a:b]) }
	pl := crc32.NewIEEE()
	var buf [ega.PageSize]byte
	for p := 0; p < 4; p++ {
		for i := range buf {
			buf[i] = v.Plane(p, uint16(i))
		}
		pl.Write(buf[:])
	}
	return fmt.Sprintf("t=%d P=%s G=%s,%s,%s,%s,%s D=%08x V=%08x",
		int64(math.Round(now*1e6)), h(0, 0x272), h(0x466, 0x470), h(0x55A, 0x560), h(0x615, 0x61C), h(0x1067, 0x106A), h(0x24F2, 0x24F8),
		crc32.ChecksumIEEE(ds), pl.Sum32())
}

func writeWAV(path string, pcm []float32) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	n := uint32(len(pcm) * 2)
	hdr := []any{
		[4]byte{'R', 'I', 'F', 'F'}, 36 + n, [4]byte{'W', 'A', 'V', 'E'},
		[4]byte{'f', 'm', 't', ' '}, uint32(16), uint16(1), uint16(1), uint32(sfx.Rate), uint32(sfx.Rate * 2), uint16(2), uint16(16),
		[4]byte{'d', 'a', 't', 'a'}, n,
	}
	for _, v := range hdr {
		if err := binary.Write(w, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	for _, v := range pcm {
		s := int16(math.Max(-1, math.Min(1, float64(v))) * 32767)
		if err := binary.Write(w, binary.LittleEndian, s); err != nil {
			return err
		}
	}
	return w.Flush()
}
