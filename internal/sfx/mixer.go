// Package sfx синтезирует звук PC-спикера: меандр частоты, заданной делителем
// канала 2 таймера.
//
// События задаются во «времени игры» (секунды от старта). Микшер переводит
// его в номер отсчёта с постоянной задержкой, поэтому звук и картинка
// остаются синхронными независимо от размера аудиобуфера.
package sfx

import (
	"encoding/binary"
	"math"
	"sort"
	"sync"
)

// Rate — частота дискретизации вывода.
const Rate = 44100

// pitClock — тактовая частота таймера PC, по которой sound() вычислял делитель.
const pitClock = 1193180

type toneEvent struct {
	at   int64
	freq float64
}

// Mixer — источник аудиопотока (float32, стерео) для Ebiten.
type Mixer struct {
	mu     sync.Mutex
	pos    int64 // сколько отсчётов уже отдано аудиопотоку
	margin int64 // запас перед позицией потока для новых событий
	slack  int64 // насколько события могут опережать позицию потока
	volume float32

	shift   int64 // поправка между временем игры и позицией аудиопотока
	synced  bool  // поправка уже выставлялась
	offline bool  // рендер без реального вывода (время игры = время звука)

	tones []toneEvent
	freq  float64
	phase float64

	lp, hpIn, hpOut float32
}

// NewMixer создаёт микшер для вывода в реальном времени. Звук, начатый
// в момент «сейчас», ставится сразу за уже отданной аудиопотоку частью
// (с запасом margin секунд), поэтому отстаёт от картинки только на размер
// буфера воспроизведения. bufferSize — размер буфера плеера в секундах:
// между его чтениями время игры уходит вперёд на эту величину.
func NewMixer(margin, bufferSize float64) *Mixer {
	return &Mixer{
		margin: int64(margin * Rate),
		slack:  int64((margin + bufferSize*1.5) * Rate),
		volume: 0.35,
	}
}

func (m *Mixer) frame(t float64) int64 {
	return int64(t*Rate) + m.shift
}

// NewOfflineMixer создаёт микшер для рендера в файл: время игры совпадает
// с временем звука, синхронизация не нужна.
func NewOfflineMixer() *Mixer {
	return &Mixer{volume: 0.35, offline: true}
}

// Sync привязывает время игры t к позиции аудиопотока. Пока соответствие
// держится в допустимых пределах, оно не меняется (так сохраняются
// интервалы между звуками); если поток ушёл вперёд или отстал (например,
// окно было свёрнуто), «сейчас» снова ставится сразу за позицией потока.
func (m *Mixer) Sync(t float64) {
	if m.offline {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	target := m.frame(t)
	if !m.synced || target < m.pos || target > m.pos+m.slack {
		m.shift += m.pos + m.margin - target
		m.synced = true
	}
}

// SetVolume задаёт громкость 0..1.
func (m *Mixer) SetVolume(v float64) {
	m.mu.Lock()
	m.volume = float32(v)
	m.mu.Unlock()
}

// Tone включает спикер на частоте freq (Гц) в момент t; freq<=0 — выключить.
// Частота округляется так же, как делитель таймера в sound() Turbo C.
func (m *Mixer) Tone(t, freq float64) {
	f := 0.0
	if freq > 18 {
		div := math.Floor(pitClock / freq)
		if div >= 1 {
			f = pitClock / div
		}
	}
	m.mu.Lock()
	ev := toneEvent{at: m.frame(t), freq: f}
	n := len(m.tones)
	if n > 0 && m.tones[n-1].at > ev.at {
		// события обычно приходят по порядку; на всякий случай поддерживаем сортировку
		i := sort.Search(n, func(i int) bool { return m.tones[i].at > ev.at })
		m.tones = append(m.tones, toneEvent{})
		copy(m.tones[i+1:], m.tones[i:])
		m.tones[i] = ev
	} else {
		m.tones = append(m.tones, ev)
	}
	m.mu.Unlock()
}

// Divisor включает спикер с делителем div канала 2 таймера в момент t; div=0 — выключить.
func (m *Mixer) Divisor(t float64, div uint16) {
	f := 0.0
	if div != 0 {
		f = pitClock / float64(div)
	}
	m.mu.Lock()
	ev := toneEvent{at: m.frame(t), freq: f}
	n := len(m.tones)
	if n > 0 && m.tones[n-1].at > ev.at {
		i := sort.Search(n, func(i int) bool { return m.tones[i].at > ev.at })
		m.tones = append(m.tones, toneEvent{})
		copy(m.tones[i+1:], m.tones[i:])
		m.tones[i] = ev
	} else {
		m.tones = append(m.tones, ev)
	}
	m.mu.Unlock()
}

// polyBLEP — поправка для подавления наложения спектра у меандра.
func polyBLEP(t, dt float64) float64 {
	if t < dt {
		t /= dt
		return t + t - t*t - 1
	}
	if t > 1-dt {
		t = (t - 1) / dt
		return t*t + t + t + 1
	}
	return 0
}

// Render заполняет out монофоническими отсчётами (используется и для тестов).
func (m *Mixer) Render(out []float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range out {
		for len(m.tones) > 0 && m.tones[0].at <= m.pos {
			m.freq = m.tones[0].freq
			m.tones = m.tones[1:]
		}
		var v float64
		if m.freq > 0 && m.freq < 20000 {
			dt := m.freq / Rate
			m.phase += dt
			if m.phase >= 1 {
				m.phase -= 1
			}
			sq := 1.0
			if m.phase >= 0.5 {
				sq = -1
			}
			sq += polyBLEP(m.phase, dt)
			sq -= polyBLEP(math.Mod(m.phase+0.5, 1), dt)
			v = sq * 0.6
		}
		x := float32(v)
		// маленький динамик: срез низких (~60 Гц) и мягкий срез высоких (~7 кГц)
		hp := 0.991 * (m.hpOut + x - m.hpIn)
		m.hpIn, m.hpOut = x, hp
		m.lp += 0.62 * (hp - m.lp)
		out[i] = m.lp * m.volume
		m.pos++
	}
}

// Read реализует io.Reader: стерео float32 little-endian.
func (m *Mixer) Read(p []byte) (int, error) {
	frames := len(p) / 8
	if frames == 0 {
		return 0, nil
	}
	buf := make([]float32, frames)
	m.Render(buf)
	for i, v := range buf {
		bits := math.Float32bits(v)
		binary.LittleEndian.PutUint32(p[i*8:], bits)
		binary.LittleEndian.PutUint32(p[i*8+4:], bits)
	}
	return frames * 8, nil
}
