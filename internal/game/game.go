// Package game — перенос программы CD-MAN 2.0 (Creative Dimensions, 1992).
//
// Оригинал написан на ассемблере и хранит всё состояние в одном сегменте
// данных. Порт держит этот сегмент как есть — массив байт, загруженный из
// CDMAN2.EXE, — и повторяет процедуры оригинала поверх него. Поэтому
// сохраняются все особенности оригинала: состояние игроков сохраняется
// копированием области памяти, таблица рекордов пишется в HIGHSC.CDM в
// исходном формате, а состояние игры можно побайтно сравнить с оригиналом,
// исполняемым в эмуляторе. Имена процедур даны по смыслу, в комментариях
// указаны их адреса в сегменте кода оригинала (CS:смещение).
//
// Игра выполняется последовательно в отдельной горутине. Время в ней течёт за
// счёт циклов задержки оригинала: каждая их итерация стоит столько же, сколько
// на эталонной машине (2 млн команд в секунду), а часы переводят это в
// реальные паузы.
package game

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/therox/go-cdman/internal/ega"
)

// Clock — время игры.
type Clock interface {
	// Now возвращает время игры в секундах.
	Now() float64
	// Advance продвигает время игры на sec секунд (с реальным ожиданием, если нужно).
	Advance(sec float64)
}

// Input — буфер клавиатуры BIOS.
type Input interface {
	// Peek — INT 16h, AH=1.
	Peek() (uint16, bool)
	// Read — INT 16h, AH=0 без ожидания.
	Read() (uint16, bool)
}

// Sound — PC-спикер.
type Sound interface {
	// Sync сообщает текущее время игры, чтобы звук не расходился с картинкой.
	Sync(t float64)
	// Divisor включает спикер с делителем таймера div (0 — выключить).
	Divisor(t float64, div uint16)
}

// Files — файлы игры по именам DOS.
type Files interface {
	Read(name string) ([]byte, error)
	Write(name string, data []byte) error
}

// Скорость эталонной машины: команд в секунду.
const baseIPS = 2e6

// Сегменты памяти DOS, которые оригинал распределял при запуске
// (значения — как при загрузке программы с сегмента 1010h).
const (
	segInit = 0x028A // копия DS:0..0x270 на момент запуска
	segP1   = 0x028C // состояние игроков (DS:0..0x22E)
	segP2   = 0x028E
	segP3   = 0x0290
	segP4   = 0x0292
	segBuf  = 0x0294 // буфер файлов

	bufSeg = 0x243A

	initSize   = 0x271
	playerSize = 0x22F
	bufSize    = 0x30000
)

// Game — состояние программы.
type Game struct {
	v   *ega.Card
	clk Clock
	in  Input
	snd Sound
	fs  Files

	// ds — сегмент данных (64 КБ, как в реальном режиме: за данными следует код).
	ds []byte

	// saves — блоки памяти DOS, в которые копируется состояние (по адресу
	// переменной с номером сегмента).
	saves map[uint16][]byte
	// buf — буфер файлов, начинается с сегмента bufSeg; переменная [0294h]
	// указывает текущее начало (оригинал временно сдвигал его).
	buf []byte

	files [8]*dosFile

	speedMu sync.Mutex
	speed   float64

	quitReq atomic.Bool

	timeShift float64   // время суток на момент запуска, с
	date      time.Time // дата для INT 21h, AH=2Ah

	// отладка и сверка
	onTick    func(g *Game)
	onCall    func(addr uint16)
	onLevel   func()
	fixedSeed bool
}

// New создаёт игру. ds — сегмент данных из CDMAN2.EXE (с кодом, до 64 КБ),
// start — момент запуска (задаёт время суток и дату).
func New(v *ega.Card, clk Clock, in Input, snd Sound, fs Files, ds []byte, start time.Time) *Game {
	g := &Game{
		v: v, clk: clk, in: in, snd: snd, fs: fs,
		ds:    make([]byte, 0x10000),
		saves: map[uint16][]byte{},
		buf:   make([]byte, bufSize),
		speed: 1,
		date:  start,
	}
	copy(g.ds, ds)
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	g.timeShift = start.Sub(day).Seconds()
	return g
}

// SetSpeed задаёт множитель скорости относительно эталонной машины.
func (g *Game) SetSpeed(v float64) {
	g.speedMu.Lock()
	g.speed = math.Max(0.05, math.Min(v, 50))
	g.speedMu.Unlock()
}

func (g *Game) speedFactor() float64 {
	g.speedMu.Lock()
	defer g.speedMu.Unlock()
	return g.speed
}

// OnCall задаёт функцию, вызываемую при входе в некоторые процедуры
// оригинала (по их адресу): L_0992 — шаг мигания надписи «Player»,
// L_1bfd — шаг ожидания клавиши под таблицами, L_1dcf — ввод имени.
// Нужна автономным прогонам, чтобы нажимать клавиши в тех же местах,
// что и в эмуляторе оригинала.
func (g *Game) OnCall(f func(addr uint16)) { g.onCall = f }

func (g *Game) call(addr uint16) {
	if g.onCall != nil {
		g.onCall(addr)
	}
}

// OnLevel задаёт функцию, вызываемую перед началом каждого этапа
// (метки L_02bc, L_03a6, L_0465, L_0642 оригинала; для автономных прогонов).
func (g *Game) OnLevel(f func()) { g.onLevel = f }

// Poke записывает слово в сегмент данных (для автономных прогонов).
func (g *Game) Poke(a, v uint16) { g.sw(a, v) }

func (g *Game) levelHook() {
	if g.onLevel != nil {
		g.onLevel()
	}
}

// OnTick задаёт функцию, вызываемую в начале каждого игрового тика (для сверки с оригиналом).
func (g *Game) OnTick(f func(g *Game)) { g.onTick = f }

// Debug задаёт отладочные параметры: fixedSeed — генератор всегда
// засевается числом 12345 (как во время демонстрации), игра становится
// детерминированной; screen > 0 — игра начинается с этого экрана (1..5).
func (g *Game) Debug(fixedSeed bool, screen int) {
	g.fixedSeed = fixedSeed
	if screen > 0 {
		g.sw(vStartScr, uint16(screen-1)*2)
	}
}

// DS возвращает сегмент данных (для отладки и сверки).
func (g *Game) DS() []byte { return g.ds }

// ---- доступ к сегменту данных ----

func (g *Game) b(a uint16) byte       { return g.ds[a] }
func (g *Game) sb(a uint16, v byte)   { g.ds[a] = v }
func (g *Game) w(a uint16) uint16     { return uint16(g.ds[a]) | uint16(g.ds[a+1])<<8 }
func (g *Game) sw(a uint16, v uint16) { g.ds[a] = byte(v); g.ds[a+1] = byte(v >> 8) }
func (g *Game) incw(a uint16)         { g.sw(a, g.w(a)+1) }
func (g *Game) decw(a uint16) uint16  { v := g.w(a) - 1; g.sw(a, v); return v }
func (g *Game) addw(a, d uint16)      { g.sw(a, g.w(a)+d) }

// bufBase — смещение сегмента [0294h] в буфере файлов.
func (g *Game) bufBase() int {
	return int(g.w(segBuf)-bufSeg) * 16
}

// ---- время ----

// spin учитывает n команд эталонной машины.
func (g *Game) spin(n float64) {
	if n > 0 {
		g.clk.Advance(n / (baseIPS * g.speedFactor()))
	}
}

// timeOfDay — время суток в секундах.
func (g *Game) timeOfDay() float64 {
	return g.timeShift + g.clk.Now()
}

// dosTime — INT 21h, AH=2Ch: CH часы, CL минуты, DH секунды, DL сотые.
func (g *Game) dosTime() (ch, cl, dh, dl byte) {
	t := g.timeOfDay()
	s := int64(t)
	hund := int64((t - float64(s)) * 100)
	return byte((s / 3600) % 24), byte((s / 60) % 60), byte(s % 60), byte(hund)
}

// dosDate — INT 21h, AH=2Ah: CX год, DH месяц, DL день.
func (g *Game) dosDate() (cx uint16, dh, dl byte) {
	d := g.date.Add(time.Duration(g.clk.Now() * float64(time.Second)))
	return uint16(d.Year()), byte(d.Month()), byte(d.Day())
}

// biosTicks — INT 1Ah, AH=0: тики таймера (18,2 Гц) с полуночи.
func (g *Game) biosTicks() uint32 {
	return uint32(g.timeOfDay() * 1193182 / 65536)
}

// ---- нелокальные переходы ----

// toMain — переход на L_0127 оригинала: «mov sp, [278h]», возврат в главный
// цикл программы из любой глубины.
type toMain struct{}

// exitDOS — завершение программы (INT 21h, AH=4Ch).
type exitDOS struct{}

// quitNow — окно закрыто: выйти, как по пункту Quit (с сохранением рекордов).
type quitNow struct{}

// RequestQuit просит игру завершиться как по пункту меню Quit: учесть время
// сеанса и сохранить таблицу рекордов. Игра выходит при ближайшем опросе
// клавиатуры.
func (g *Game) RequestQuit() { g.quitReq.Store(true) }
