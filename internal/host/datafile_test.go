package host

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func gameDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("CDMAN_DATA")
	if dir == "" {
		dir = "../../Source"
	}
	if err := Check(dir); err != nil {
		t.Skip("нет файлов игры (задайте CDMAN_DATA):", err)
	}
	return dir
}

// Перетащенная папка с игрой копируется в каталог настроек и потом находится сама.
func TestInstallFiles(t *testing.T) {
	src := gameDir(t)
	configDirOverride = t.TempDir()
	defer func() { configDirOverride = "" }()

	fsys := fstest.MapFS{}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		// имена в нижнем регистре и во вложенной папке
		fsys["game/sub/"+lower(e.Name())] = &fstest.MapFile{Data: b}
	}
	dir, err := InstallFiles(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if dir != configDirOverride {
		t.Fatalf("установлено в %s", dir)
	}
	if err := Check(dir); err != nil {
		t.Fatal(err)
	}
}

func TestInstallFilesMissing(t *testing.T) {
	configDirOverride = t.TempDir()
	defer func() { configDirOverride = "" }()
	if _, err := InstallFiles(fstest.MapFS{"readme.txt": {Data: []byte("x")}}); !errors.Is(err, ErrNoData) {
		t.Fatalf("без файлов игры: %v", err)
	}
	if _, err := InstallFiles(fstest.MapFS{"CDMAN2.EXE": {Data: []byte("x")}}); !errors.Is(err, ErrNoData) {
		t.Fatalf("без картинок: %v", err)
	}
}

// Таблица рекордов пишется в каталог настроек и читается оттуда в первую очередь.
func TestHiScoreInConfig(t *testing.T) {
	src := gameDir(t)
	f := &Files{Dir: src, Config: t.TempDir()}
	if err := f.Write(HiScoreName, []byte("таблица")); err != nil {
		t.Fatal(err)
	}
	b, err := f.Read("highsc.cdm")
	if err != nil || string(b) != "таблица" {
		t.Fatalf("прочитано %q, %v", b, err)
	}
	if _, err := f.Read("TITLE.CDM"); err != nil {
		t.Fatal(err)
	}
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
