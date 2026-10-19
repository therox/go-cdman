package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/therox/go-cdman/internal/dat"
	"github.com/therox/go-cdman/internal/ega"
	"github.com/therox/go-cdman/internal/game"
	"github.com/therox/go-cdman/internal/host"
	"github.com/therox/go-cdman/internal/sfx"
)

const windowTitle = "CD-MAN"

// Задержка автоповтора клавиш PC: 0,5 с, затем ~11 нажатий в секунду (в тиках по 1/60 с).
const (
	repeatDelay = 30
	repeatEvery = 5
)

// specialKeys — служебные клавиши окна и их коды PC.
var specialKeys = map[ebiten.Key]string{
	ebiten.KeyEscape:      "esc",
	ebiten.KeyEnter:       "enter",
	ebiten.KeyNumpadEnter: "enter",
	ebiten.KeyBackspace:   "bksp",
	ebiten.KeyTab:         "tab",
	ebiten.KeyArrowUp:     "up",
	ebiten.KeyArrowDown:   "down",
	ebiten.KeyArrowLeft:   "left",
	ebiten.KeyArrowRight:  "right",
	ebiten.KeyHome:        "home",
	ebiten.KeyEnd:         "end",
	ebiten.KeyPageUp:      "pgup",
	ebiten.KeyPageDown:    "pgdn",
	ebiten.KeyInsert:      "ins",
	ebiten.KeyDelete:      "del",
	ebiten.KeyF1:          "f1",
}

// Стрелки дополнительной клавиатуры (при выключенном NumLock на PC
// они давали те же скан-коды, что и стрелки).
var numpadArrows = map[ebiten.Key]string{
	ebiten.KeyNumpad8: "up",
	ebiten.KeyNumpad2: "down",
	ebiten.KeyNumpad4: "left",
	ebiten.KeyNumpad6: "right",
}

// gamepadKeys — кнопки геймпада, которые нажимают клавиши.
var gamepadKeys = map[ebiten.StandardGamepadButton]string{
	ebiten.StandardGamepadButtonLeftTop:         "up",
	ebiten.StandardGamepadButtonLeftBottom:      "down",
	ebiten.StandardGamepadButtonLeftLeft:        "left",
	ebiten.StandardGamepadButtonLeftRight:       "right",
	ebiten.StandardGamepadButtonRightBottom:     "enter",
	ebiten.StandardGamepadButtonCenterRight:     "enter",
	ebiten.StandardGamepadButtonRightRight:      "esc",
	ebiten.StandardGamepadButtonCenterLeft:      "esc",
	ebiten.StandardGamepadButtonRightLeft:       "space",
	ebiten.StandardGamepadButtonRightTop:        "tab",
	ebiten.StandardGamepadButtonFrontBottomLeft: "bksp",
}

type app struct {
	v     *ega.Card
	kb    *host.Keyboard
	g     *game.Game
	done  chan struct{}
	mixer *sfx.Mixer

	frame *ebiten.Image
	pre   *ebiten.Image
	preK  [2]int
	pix   []byte
	runes []rune
	speed float64

	pads      []ebiten.GamepadID
	stickHeld map[ebiten.GamepadID]string
	stickTime map[ebiten.GamepadID]int

	// показ кадров с задержкой, чтобы картинка совпадала с запаздывающим
	// звуком (Bluetooth-наушники и колонки)
	settings host.Settings
	history  []timedFrame
	spare    []*ega.Frame
	ticks    int

	// отладка: сохранить кадр окна через 3 с после запуска и выйти
	capturePath string
	captureAt   time.Time
	captured    bool
}

// timedFrame — снимок экрана и момент, когда он был снят.
type timedFrame struct {
	at    time.Time
	frame *ega.Frame
}

// Пределы и шаг компенсации задержки звука.
const (
	maxAudioDelayMs  = 800
	audioDelayStepMs = 20
)

func runWindow(files *host.Files, dataErr error, speed, volume float64, fullscreen bool, capture string, audioDelayMs int) error {
	const audioBuffer = 25 * time.Millisecond
	mixer := sfx.NewMixer(0.005, audioBuffer.Seconds())
	mixer.SetVolume(volume)
	actx := audio.NewContext(sfx.Rate)
	player, err := actx.NewPlayerF32(mixer)
	if err != nil {
		return err
	}
	player.SetBufferSize(audioBuffer)
	player.Play()

	a := &app{
		kb:        &host.Keyboard{},
		done:      make(chan struct{}),
		mixer:     mixer,
		frame:     ebiten.NewImage(ega.W, ega.H),
		pix:       make([]byte, ega.W*ega.H*4),
		speed:     speed,
		stickHeld: map[ebiten.GamepadID]string{},
		stickTime: map[ebiten.GamepadID]int{},

		settings: host.LoadSettings(),

		capturePath: capture,
		captureAt:   time.Now().Add(3 * time.Second),
	}
	if audioDelayMs >= 0 {
		a.settings.AudioDelayMs = audioDelayMs
	}
	a.settings.AudioDelayMs = min(max(a.settings.AudioDelayMs, 0), maxAudioDelayMs)
	if files != nil {
		if err := a.start(files); err != nil {
			dataErr = err
		}
	}
	if a.g == nil {
		a.v = ega.New(ega.CyrFont)
		drawNoData(a.v, dataErr)
	}

	ebiten.SetWindowTitle(a.title())
	ebiten.SetWindowSize(960, 720)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(fullscreen)
	ebiten.SetScreenClearedEveryFrame(true)
	ebiten.SetTPS(60)
	// закрытие окна — как пункт Quit: таблица рекордов сохраняется
	ebiten.SetWindowClosingHandled(true)
	return ebiten.RunGame(a)
}

// start запускает игру, когда файлы игры есть.
func (a *app) start(files *host.Files) error {
	ds, err := files.LoadDS()
	if err != nil {
		return err
	}
	a.v = ega.New(ega.ROMFont)
	a.g = game.New(a.v, host.NewRealClock(), a.kb, a.mixer, files, ds, time.Now())
	a.g.SetSpeed(a.speed)
	if icons := windowIcons(files); icons != nil {
		ebiten.SetWindowIcon(icons)
	}
	go func() {
		defer close(a.done)
		if err := a.g.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "cdman:", err)
		}
	}()
	return nil
}

// waitData обрабатывает перетаскивание файлов игры в окно.
func (a *app) waitData() {
	dropped := ebiten.DroppedFiles()
	if dropped == nil {
		return
	}
	dir, err := host.InstallFiles(dropped)
	if err == nil {
		files := &host.Files{Dir: dir, Config: host.ConfigDir()}
		if err = a.start(files); err == nil {
			return
		}
	}
	drawNoData(a.v, err)
}

// drawNoData выводит в окне подсказку, где взять файлы игры.
func drawNoData(v *ega.Card, err error) {
	v.SetMode(3)
	v.SetBlink(false)
	v.SetCursorShape(0x2000)
	v.SetPalette([16]byte{0, 1, 2, 3, 4, 5, 0x14, 7, 0x38, 0x39, 0x3A, 0x3B, 0x3C, 0x3D, 0x3E, 0x3F})
	for i := uint16(0); i < ega.Cols*ega.Rows*2; i += 2 {
		v.TextW(i, 0x1F20)
	}
	line := func(row int, attr byte, text string) {
		b := cp866(text)
		col := (ega.Cols - len(b)) / 2
		for i, c := range b {
			v.TextW(uint16((row*ega.Cols+col+i)*2), uint16(attr)<<8|uint16(c))
		}
	}
	switch {
	case err == nil || errors.Is(err, host.ErrNoData):
		line(7, 0x1E, "Не найдены файлы игры CD-MAN 2.0")
	case errors.Is(err, dat.ErrFormat):
		line(7, 0x1E, "Этот CDMAN2.EXE не подходит (нужна версия 2.0)")
	default:
		line(7, 0x1E, "Не удалось прочитать файлы игры")
		msg := []rune(err.Error())
		if len(msg) > 76 {
			msg = append([]rune("..."), msg[len(msg)-73:]...)
		}
		line(17, 0x1C, string(msg))
	}
	line(10, 0x1F, "Для игры нужны файлы CD-MAN 2.0 (Creative Dimensions, 1992):")
	line(11, 0x1F, "CDMAN2.EXE и все файлы *.CDM.")
	line(13, 0x1F, "Перетащите папку с ними в это окно")
	line(14, 0x1F, "или положите их рядом с программой.")
}

// cp866 переводит строку в кодировку шрифта подсказки.
func cp866(s string) []byte {
	var b []byte
	for _, r := range s {
		switch {
		case r < 0x80:
			b = append(b, byte(r))
		case r >= 'А' && r <= 'п':
			b = append(b, byte(r-'А'+0x80))
		case r >= 'р' && r <= 'я':
			b = append(b, byte(r-'р'+0xE0))
		case r == 'Ё':
			b = append(b, 0xF0)
		case r == 'ё':
			b = append(b, 0xF1)
		case r == '«' || r == '»':
			b = append(b, '"')
		case r == '—':
			b = append(b, '-')
		default:
			b = append(b, '?')
		}
	}
	return b
}

func (a *app) title() string {
	t := windowTitle
	if math.Abs(a.speed-1) > 1e-9 {
		t += fmt.Sprintf(" — скорость ×%.2g", a.speed)
	}
	if a.settings.AudioDelayMs > 0 {
		t += fmt.Sprintf(" — задержка звука %d мс", a.settings.AudioDelayMs)
	}
	return t
}

func (a *app) press(name string) {
	k := host.KeyByName[name]
	a.kb.Press(k.Scan, k.ASCII)
}

func (a *app) Update() error {
	select {
	case <-a.done:
		return ebiten.Termination
	default:
	}
	if a.capturePath != "" && a.captured {
		return ebiten.Termination
	}
	if ebiten.IsWindowBeingClosed() {
		if a.g != nil {
			a.g.RequestQuit()
			select {
			case <-a.done:
			case <-time.After(2 * time.Second):
			}
		}
		return ebiten.Termination
	}
	a.ticks++
	if a.g == nil {
		a.waitData()
	}

	alt := ebiten.IsKeyPressed(ebiten.KeyAlt)
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl)
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) || (alt && inpututil.IsKeyJustPressed(ebiten.KeyEnter)) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF7) || inpututil.IsKeyJustPressed(ebiten.KeyF8) || inpututil.IsKeyJustPressed(ebiten.KeyF9) {
		switch {
		case inpututil.IsKeyJustPressed(ebiten.KeyF7):
			a.speed = math.Max(0.25, a.speed/1.25)
		case inpututil.IsKeyJustPressed(ebiten.KeyF8):
			a.speed = math.Min(8, a.speed*1.25)
		default:
			a.speed = 1
		}
		if a.g != nil {
			a.g.SetSpeed(a.speed)
		}
		ebiten.SetWindowTitle(a.title())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) || inpututil.IsKeyJustPressed(ebiten.KeyF6) {
		step := audioDelayStepMs
		if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
			step = -step
		}
		a.settings.AudioDelayMs = min(max(a.settings.AudioDelayMs+step, 0), maxAudioDelayMs)
		_ = host.SaveSettings(a.settings)
		ebiten.SetWindowTitle(a.title())
	}
	if a.g == nil {
		return nil
	}

	// Ctrl+Q — немедленный выход, как в оригинале
	if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		a.press("ctrl-q")
		return nil
	}
	// служебные клавиши с автоповтором
	for k, name := range specialKeys {
		if k == ebiten.KeyEnter && alt {
			continue
		}
		if repeatPressed(k) {
			a.press(name)
		}
	}
	for k, name := range numpadArrows {
		if repeatPressed(k) {
			a.press(name)
		}
	}
	// печатные символы — с учётом раскладки клавиатуры
	if !ctrl {
		a.runes = ebiten.AppendInputChars(a.runes[:0])
		for _, r := range a.runes {
			if r >= 0x20 && r < 0x7F {
				k := host.CharKey(byte(r))
				a.kb.Press(k.Scan, k.ASCII)
			}
		}
	}
	a.gamepads()
	return nil
}

// gamepads переводит кнопки и левый стик геймпада в нажатия клавиш.
func (a *app) gamepads() {
	a.pads = ebiten.AppendGamepadIDs(a.pads[:0])
	for _, id := range a.pads {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		for b, name := range gamepadKeys {
			d := inpututil.StandardGamepadButtonPressDuration(id, b)
			if d == 1 || (d > repeatDelay && (d-repeatDelay)%repeatEvery == 0) {
				a.press(name)
			}
		}
		x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		dir := ""
		switch {
		case math.Abs(x) < 0.5 && math.Abs(y) < 0.5:
		case math.Abs(x) > math.Abs(y) && x < 0:
			dir = "left"
		case math.Abs(x) > math.Abs(y):
			dir = "right"
		case y < 0:
			dir = "up"
		default:
			dir = "down"
		}
		if dir != a.stickHeld[id] {
			a.stickHeld[id] = dir
			a.stickTime[id] = 0
		}
		if dir != "" {
			a.stickTime[id]++
			d := a.stickTime[id]
			if d == 1 || (d > repeatDelay && (d-repeatDelay)%repeatEvery == 0) {
				a.press(dir)
			}
		}
	}
}

// repeatPressed — нажатие клавиши с автоповтором, как у клавиатуры PC.
func repeatPressed(k ebiten.Key) bool {
	d := inpututil.KeyPressDuration(k)
	return d == 1 || (d > repeatDelay && (d-repeatDelay)%repeatEvery == 0)
}

// delayedFrame снимает текущий кадр и возвращает тот, что был на экране
// AudioDelayMs миллисекунд назад.
func (a *app) delayedFrame() *ega.Frame {
	now := time.Now()
	var f *ega.Frame
	if n := len(a.spare); n > 0 {
		f, a.spare = a.spare[n-1], a.spare[:n-1]
	} else {
		f = &ega.Frame{}
	}
	// курсор мигает раз в 16 кадров, как на настоящем адаптере
	a.v.Capture(f, (a.ticks/16)%2 == 0)
	a.history = append(a.history, timedFrame{now, f})

	cutoff := now.Add(-time.Duration(a.settings.AudioDelayMs) * time.Millisecond)
	// самый свежий кадр не позже cutoff; более старые больше не понадобятся
	k := 0
	for k+1 < len(a.history) && !a.history[k+1].at.After(cutoff) {
		k++
	}
	for _, old := range a.history[:k] {
		a.spare = append(a.spare, old.frame)
	}
	a.history = append(a.history[:0], a.history[k:]...)
	return a.history[0].frame
}

func (a *app) Draw(screen *ebiten.Image) {
	a.delayedFrame().RGBA(a.pix)
	a.frame.WritePixels(a.pix)

	sw, sh := screen.Bounds().Dx(), screen.Bounds().Dy()
	screen.Fill(color.Black)
	// изображение EGA растягивается до пропорций 4:3, как на мониторе того времени
	w := math.Min(float64(sw), float64(sh)*4/3)
	h := w * 3 / 4
	// «чёткий» билинейный: целочисленное увеличение ближайшим соседом,
	// затем плавное масштабирование до нужного размера
	kx := max(1, int(math.Ceil(w/ega.W)))
	ky := max(1, int(math.Ceil(h/ega.H)))
	if a.pre == nil || a.preK != [2]int{kx, ky} {
		if a.pre != nil {
			a.pre.Deallocate()
		}
		a.pre = ebiten.NewImage(ega.W*kx, ega.H*ky)
		a.preK = [2]int{kx, ky}
	}
	var op ebiten.DrawImageOptions
	op.GeoM.Scale(float64(kx), float64(ky))
	a.pre.DrawImage(a.frame, &op)

	op = ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(w/float64(ega.W*kx), h/float64(ega.H*ky))
	op.GeoM.Translate((float64(sw)-w)/2, (float64(sh)-h)/2)
	screen.DrawImage(a.pre, &op)

	if a.capturePath != "" && !a.captured && time.Now().After(a.captureAt) {
		saveScreen(screen, a.capturePath)
		a.captured = true
	}
}

// saveScreen сохраняет кадр окна в PNG (для отладки отрисовки).
func saveScreen(screen *ebiten.Image, path string) {
	b := screen.Bounds()
	img := image.NewRGBA(b)
	screen.ReadPixels(img.Pix)
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	_ = png.Encode(f, img)
}

func (a *app) Layout(outsideWidth, outsideHeight int) (int, int) {
	s := ebiten.Monitor().DeviceScaleFactor()
	return int(float64(outsideWidth) * s), int(float64(outsideHeight) * s)
}

// windowIcons строит значки окна из кадра CD-Man в GRAFIX1.CDM.
func windowIcons(files *host.Files) []image.Image {
	raw, err := files.Read("GRAFIX1.CDM")
	if err != nil {
		return nil
	}
	img, err := dat.DecodeCDM(raw)
	if err != nil {
		return nil
	}
	// первый кадр листа: CD-Man смотрит вправо, 32×24 точки на чёрном фоне
	const sw, sh = 32, 24
	bg := img.Pix[0]
	var icons []image.Image
	for _, size := range []int{16, 32, 48, 64} {
		ic := image.NewRGBA(image.Rect(0, 0, size, size))
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				sx := x * sw / size
				sy := y*sw/size - (sw-sh)/2
				if sy < 0 || sy >= sh {
					continue
				}
				c := img.Pix[sy*dat.W+sx]
				if c == bg {
					continue
				}
				ic.Set(x, y, ega.Color(img.Pal[c]))
			}
		}
		icons = append(icons, ic)
	}
	return icons
}
