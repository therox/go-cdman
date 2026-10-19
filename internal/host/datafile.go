package host

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/therox/go-cdman/internal/dat"
)

// ExeName — программа оригинальной игры, из неё берётся сегмент данных.
const ExeName = "CDMAN2.EXE"

// HiScoreName — файл таблицы рекордов и статистики.
const HiScoreName = "HIGHSC.CDM"

// RequiredFiles — файлы оригинала, без которых игра не запустится
// (HELP.CDM и HIGHSC.CDM необязательны, как и в оригинале).
var RequiredFiles = []string{
	ExeName, "CHARTAB.CDM", "TITLE.CDM", "WOSHI.CDM", "DEMO.CDM",
	"SCREEN1.CDM", "SCREEN2.CDM", "SCREEN3.CDM", "SCREEN4.CDM", "SCREEN5.CDM",
	"GRAPHIC1.CDM", "GRAPHIC2.CDM", "GRAPHIC3.CDM", "GRAPHIC4.CDM", "GRAPHIC5.CDM",
	"GRAFIX1.CDM", "GRAFIX2.CDM", "GRAFIX3.CDM", "GRAFIX4.CDM", "GRAFIX5.CDM",
}

// optionalFiles — файлы, которые копируются при установке, если они есть.
var optionalFiles = []string{"HELP.CDM", HiScoreName}

// ErrNoData — файлы игры не найдены.
var ErrNoData = errors.New("не найдены файлы игры CD-MAN 2.0 (" + ExeName + " и *.CDM)")

// configDirOverride подменяет каталог настроек (для тестов).
var configDirOverride string

// ConfigDir возвращает каталог игры в настройках пользователя
// (там хранятся рекорды, настройки и скопированные файлы игры).
func ConfigDir() string {
	if configDirOverride != "" {
		return configDirOverride
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(dir, "cdman")
}

// Files — доступ к файлам игры по их именам DOS.
type Files struct {
	// Dir — каталог с файлами оригинала.
	Dir string
	// Config — каталог для изменяемых файлов (HIGHSC.CDM).
	Config string
}

// Read читает файл игры. HIGHSC.CDM сначала ищется в каталоге настроек.
func (f *Files) Read(name string) ([]byte, error) {
	if strings.EqualFold(name, HiScoreName) && f.Config != "" {
		if p := findInDir(f.Config, name); p != "" {
			return os.ReadFile(p)
		}
	}
	p := findInDir(f.Dir, name)
	if p == "" {
		return nil, fmt.Errorf("%s: %w", name, fs.ErrNotExist)
	}
	return os.ReadFile(p)
}

// Write записывает изменяемый файл игры в каталог настроек.
func (f *Files) Write(name string, data []byte) error {
	dir := f.Config
	if dir == "" {
		dir = f.Dir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, name+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, name))
}

// LoadDS читает CDMAN2.EXE и возвращает начальный сегмент данных программы.
func (f *Files) LoadDS() ([]byte, error) {
	raw, err := f.Read(ExeName)
	if err != nil {
		return nil, err
	}
	return dat.ParseEXE(raw)
}

// Check проверяет, что в каталоге есть все нужные файлы и программа подходящей версии.
func Check(dir string) error {
	for _, n := range RequiredFiles {
		if findInDir(dir, n) == "" {
			return fmt.Errorf("%w: в %s нет %s", ErrNoData, dir, n)
		}
	}
	f := &Files{Dir: dir}
	_, err := f.LoadDS()
	return err
}

// FindDataDir ищет каталог с файлами игры: рядом с программой (и в его
// подкаталоге Source), в текущем каталоге и в каталоге настроек.
func FindDataDir() (string, error) {
	var dirs []string
	if exe, err := os.Executable(); err == nil {
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		dirs = append(dirs, filepath.Dir(exe), filepath.Join(filepath.Dir(exe), "Source"))
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd, filepath.Join(wd, "Source"))
	}
	dirs = append(dirs, ConfigDir())
	var firstErr error
	for _, d := range dirs {
		if findInDir(d, ExeName) == "" {
			continue
		}
		err := Check(d)
		if err == nil {
			return d, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	if firstErr != nil {
		return "", firstErr
	}
	return "", ErrNoData
}

func findInDir(dir, name string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// InstallFiles копирует файлы игры из fsys (например, перетащенной в окно
// папки) в каталог настроек, чтобы в следующий раз они нашлись сами.
// Возвращает каталог, куда они скопированы.
func InstallFiles(fsys fs.FS) (string, error) {
	found := map[string]string{}
	want := map[string]bool{}
	for _, n := range append(append([]string{}, RequiredFiles...), optionalFiles...) {
		want[strings.ToUpper(n)] = true
	}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		u := strings.ToUpper(d.Name())
		if want[u] && found[u] == "" {
			found[u] = p
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	for _, n := range RequiredFiles {
		if found[n] == "" {
			if len(found) == 0 {
				return "", ErrNoData
			}
			return "", fmt.Errorf("%w: среди перетащенных файлов нет %s", ErrNoData, n)
		}
	}
	raw, err := fs.ReadFile(fsys, found[ExeName])
	if err != nil {
		return "", err
	}
	if _, err := dat.ParseEXE(raw); err != nil {
		return "", err
	}
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	for name, p := range found {
		if name == HiScoreName && findInDir(dir, name) != "" {
			continue // таблицу рекордов игрока не затираем
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}
