package host

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Settings — настройки, которые запоминаются между запусками.
type Settings struct {
	// AudioDelayMs — на сколько миллисекунд задерживать картинку, чтобы она
	// совпадала со звуком (нужно для Bluetooth-наушников и колонок).
	AudioDelayMs int
}

func settingsPath() string {
	return filepath.Join(ConfigDir(), "settings.txt")
}

// LoadSettings читает настройки; при отсутствии файла возвращает значения по умолчанию.
func LoadSettings() Settings {
	var st Settings
	f, err := os.Open(settingsPath())
	if err != nil {
		return st
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		key, val, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(key) == "audio-delay-ms" {
			if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
				st.AudioDelayMs = v
			}
		}
	}
	return st
}

// SaveSettings записывает настройки.
func SaveSettings(st Settings) error {
	if err := os.MkdirAll(ConfigDir(), 0o755); err != nil {
		return err
	}
	text := fmt.Sprintf("# Настройки CD-MAN\naudio-delay-ms=%d\n", st.AudioDelayMs)
	return os.WriteFile(settingsPath(), []byte(text), 0o644)
}
