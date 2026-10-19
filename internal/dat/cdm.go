package dat

import "errors"

// Картинки CD-MAN (.CDM): 16 байт палитры EGA, затем изображение 640×350,
// сжатое RLE. Плоскости идут в порядке 3, 2, 1, 0; внутри плоскости — по
// столбцам байт (80 столбцов по 350 строк). Управляющий байт n: если старший
// бит сброшен — n байт данных, иначе — повтор следующего байта (n & 7Fh) раз.
const (
	W = 640
	H = 350
)

// ErrImage — файл не похож на картинку .CDM.
var ErrImage = errors.New("файл не похож на картинку CD-MAN (.CDM)")

// Image — распакованная картинка: палитра и индексы цветов (W×H).
type Image struct {
	Pal [16]byte
	Pix []byte
}

// DecodeCDM распаковывает картинку .CDM.
func DecodeCDM(raw []byte) (*Image, error) {
	if len(raw) < 17 {
		return nil, ErrImage
	}
	img := &Image{Pix: make([]byte, W*H)}
	copy(img.Pal[:], raw[:16])
	src := raw[16:]
	si := 0
	next := func() byte {
		var b byte
		if si < len(src) {
			b = src[si]
		}
		si++
		return b
	}
	for _, p := range [4]uint{3, 2, 1, 0} {
		bh := byte(1)
		var bl byte
		for col := 0; col < 80; col++ {
			for row := 0; row < H; row++ {
				bh--
				switch {
				case bh&0x7F == 0:
					bh = next()
					bl = next()
				case bh&0x80 == 0:
					bl = next()
				}
				for x := 0; x < 8; x++ {
					if bl&(0x80>>x) != 0 {
						img.Pix[row*W+col*8+x] |= 1 << p
					}
				}
			}
		}
	}
	if si > len(src) {
		return nil, errors.New("картинка .CDM обрывается")
	}
	return img, nil
}
