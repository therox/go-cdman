package dat

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func dataDir() string {
	if d := os.Getenv("CDMAN_DATA"); d != "" {
		return d
	}
	return "../../Source"
}

func TestParseEXEBad(t *testing.T) {
	if _, err := ParseEXE([]byte("MZ не тот файл")); !errors.Is(err, ErrFormat) {
		t.Fatalf("короткий файл: %v", err)
	}
	b := make([]byte, exeSize)
	if _, err := ParseEXE(b); !errors.Is(err, ErrFormat) {
		t.Fatalf("без заголовка MZ: %v", err)
	}
}

func TestParseEXE(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(dataDir(), "CDMAN2.EXE"))
	if err != nil {
		t.Skip("нет файлов игры (задайте CDMAN_DATA):", err)
	}
	ds, err := ParseEXE(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != DSSize {
		t.Fatalf("размер сегмента данных %d", len(ds))
	}
	// лабиринт первого экрана: рамка и 19 клеток в строке
	if string(ds[0x83E:0x83E+21]) != "---------------------" || ds[0x83E+21] != '|' {
		t.Fatalf("лабиринт не на месте: %q", ds[0x83E:0x83E+24])
	}
}

// Все картинки распаковываются, и данные кончаются ровно с последней плоскостью.
func TestDecodeCDM(t *testing.T) {
	dir := dataDir()
	names := []string{"TITLE.CDM", "WOSHI.CDM"}
	for _, p := range []string{"SCREEN", "GRAPHIC", "GRAFIX"} {
		for i := 1; i <= 5; i++ {
			names = append(names, p+string(rune('0'+i))+".CDM")
		}
	}
	for _, n := range names {
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Skip("нет файлов игры (задайте CDMAN_DATA):", err)
		}
		img, err := DecodeCDM(raw)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		if _, err := DecodeCDM(raw[:len(raw)-1]); err == nil {
			t.Errorf("%s: обрезанный файл распаковался без ошибки", n)
		}
		used := map[byte]bool{}
		for _, c := range img.Pix {
			used[c] = true
		}
		if len(used) < 4 {
			t.Errorf("%s: всего %d цветов", n, len(used))
		}
	}
}
