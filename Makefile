# Сборка CD-MAN для Linux, macOS и Windows.
# Ebiten на всех трёх системах собирается без cgo, поэтому кросс-компиляция
# работает с любой машины.

APP     := cdman
DIST    := dist
LDFLAGS := -s -w
GOBUILD := CGO_ENABLED=0 go build -trimpath
# каталог с файлами игры для make run (по умолчанию программа ищет его сама)
DATA    ?=

.PHONY: build run test dist clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(APP) .

run: build
	./$(APP) $(if $(DATA),-data $(DATA))

test:
	go test ./...

dist:
	GOOS=linux   GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST)/$(APP)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST)/$(APP)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST)/$(APP)-macos-amd64 .
	GOOS=darwin  GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o $(DIST)/$(APP)-macos-arm64 .
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS) -H=windowsgui" -o $(DIST)/$(APP)-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS) -H=windowsgui" -o $(DIST)/$(APP)-windows-arm64.exe .

clean:
	rm -rf $(APP) $(DIST)
