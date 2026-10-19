package host

import (
	"time"
)

// RealClock — время игры, привязанное к реальному времени: Advance ждёт,
// пока реальное время не догонит время игры.
type RealClock struct {
	start time.Time
	vt    float64
}

// NewRealClock запускает часы.
func NewRealClock() *RealClock {
	return &RealClock{start: time.Now()}
}

// Now возвращает время игры в секундах.
func (c *RealClock) Now() float64 { return c.vt }

// Advance продвигает время игры и при необходимости спит.
func (c *RealClock) Advance(sec float64) {
	c.vt += sec
	wall := time.Since(c.start).Seconds()
	switch lead := c.vt - wall; {
	case lead > 0.002:
		time.Sleep(time.Duration(lead * float64(time.Second)))
	case lead < -0.1:
		// компьютер не успевал (или процесс стоял) — не пытаемся «догонять»
		c.vt = wall - 0.1
	}
}

// ScriptEvent — событие сценария автономного прогона.
type ScriptEvent struct {
	At float64
	Do func()
}

// ScriptClock — виртуальное время без ожидания; события сценария
// выполняются, когда время игры их достигает. Для тестов и снимков экрана.
type ScriptClock struct {
	vt     float64
	events []ScriptEvent
}

// NewScriptClock создаёт часы с упорядоченными по времени событиями.
func NewScriptClock(events []ScriptEvent) *ScriptClock {
	return &ScriptClock{events: events}
}

// Now возвращает время игры.
func (c *ScriptClock) Now() float64 { return c.vt }

// Advance продвигает время и выполняет наступившие события.
func (c *ScriptClock) Advance(sec float64) {
	c.vt += sec
	for len(c.events) > 0 && c.events[0].At <= c.vt {
		ev := c.events[0]
		c.events = c.events[1:]
		ev.Do()
	}
}
