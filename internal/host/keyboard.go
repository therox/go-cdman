// Package host — окружение игры: часы, клавиатура, файлы игры и настройки.
package host

import "sync"

// Keyboard — буфер клавиатуры BIOS: слова (скан-код<<8 | ASCII).
type Keyboard struct {
	mu   sync.Mutex
	keys []uint16
}

// Ёмкость буфера клавиатуры BIOS.
const biosBufSize = 15

// Press регистрирует нажатие: scan — скан-код, ascii — символ (0 для служебных клавиш).
func (k *Keyboard) Press(scan, ascii byte) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.keys) < biosBufSize {
		k.keys = append(k.keys, uint16(scan)<<8|uint16(ascii))
	}
}

// Peek — INT 16h, AH=1: есть ли клавиша в буфере, и какая.
func (k *Keyboard) Peek() (uint16, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.keys) == 0 {
		return 0, false
	}
	return k.keys[0], true
}

// Read — INT 16h, AH=0 без ожидания: извлечь клавишу из буфера.
func (k *Keyboard) Read() (uint16, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.keys) == 0 {
		return 0, false
	}
	v := k.keys[0]
	k.keys = k.keys[1:]
	return v, true
}
