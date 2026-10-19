// CD-MAN — порт игры Creative Dimensions (1992) для Linux, macOS и Windows.
//
// Нужны файлы оригинальной игры CD-MAN 2.0 (CDMAN2.EXE и *.CDM): они ищутся
// рядом с программой, в текущем каталоге (и в его подкаталоге Source) и в
// каталоге настроек игры либо указываются флагом -data.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/therox/go-cdman/internal/host"
)

func main() {
	dataDir := flag.String("data", "", "каталог с файлами CD-MAN 2.0 (CDMAN2.EXE и *.CDM)")
	speed := flag.Float64("speed", 1, "скорость эталонной машины (больше — быстрее)")
	fullscreen := flag.Bool("fullscreen", false, "полноэкранный режим")
	volume := flag.Float64("volume", 0.7, "громкость 0..1")
	audioDelay := flag.Int("audio-delay", -1, "задержка картинки под звук, мс (для Bluetooth-наушников; по умолчанию — сохранённая)")
	script := flag.String("script", "", "автономный прогон по сценарию (для отладки)")
	outDir := flag.String("out", ".", "каталог для снимков автономного прогона")
	wavPath := flag.String("wav", "", "записать звук автономного прогона в WAV")
	stateLog := flag.String("debug-state", "", "отладка: писать состояние игры в начале каждого тика в файл")
	capture := flag.String("debug-capture", "", "отладка: сохранить кадр окна через 3 с в PNG и выйти")
	fixedSeed := flag.Bool("debug-seed", false, "отладка: всегда засевать генератор числом 12345 (детерминированная игра)")
	startScreen := flag.Int("debug-screen", 0, "отладка: начинать игру с экрана 1..5")
	flag.Parse()

	files, err := findFiles(*dataDir)
	if *script != "" {
		if err != nil {
			fail(err)
		}
		dbg := debugOptions{fixedSeed: *fixedSeed, screen: *startScreen, stateLog: *stateLog}
		if err := runScript(files, *script, *outDir, *wavPath, *speed, dbg); err != nil {
			fail(err)
		}
		return
	}
	// без файлов игры окно всё равно открывается: в нём объясняется, где их взять
	if err := runWindow(files, err, *speed, *volume, *fullscreen, *capture, *audioDelay); err != nil {
		fail(err)
	}
}

// findFiles находит каталог с файлами игры.
func findFiles(dir string) (*host.Files, error) {
	if dir == "" {
		d, err := host.FindDataDir()
		if err != nil {
			return nil, err
		}
		dir = d
	} else if err := host.Check(dir); err != nil {
		return nil, err
	}
	return &host.Files{Dir: dir, Config: host.ConfigDir()}, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "cdman:", err)
	os.Exit(1)
}
