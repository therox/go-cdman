# CD-MAN

**English** | [Русский](README.ru.md)

A port of CD-MAN 2.0 (Creative Dimensions, 1992), a Pac-Man variation for
EGA. CD-Man eats dots in five picture mazes: a park by a castle, tropical
islands, outer space, world landmarks and a lake at night. Four monsters
chase him; energizers in the corners make them edible for a while, and
bonuses show up in the maze.

Runs on Linux, macOS and Windows.

![Gameplay](docs/screenshots/gameplay.gif)

| | |
|---|---|
| ![Title](docs/screenshots/title.png) | ![Menu](docs/screenshots/menu.png) |
| ![Park](docs/screenshots/level1.png) | ![Space](docs/screenshots/level3.png) |
| ![Landmarks](docs/screenshots/level4.png) | ![Help](docs/screenshots/help.png) |
| ![Top 10](docs/screenshots/top10.png) | ![Statistics](docs/screenshots/stats.png) |

## Building and running

Requires Go 1.25 or newer.

```sh
go build -o cdman .   # cdman.exe on Windows
./cdman
```

`make dist` builds Linux, macOS and Windows versions (amd64 and arm64) into
`dist/`. Any of these systems can build for any other.

On Linux, OpenGL (Mesa) and ALSA (`libasound`) are required. Desktop
distributions usually have them installed already.

## Game files

The port needs the files of the original CD-MAN 2.0: the program
`CDMAN2.EXE` (mazes, texts and tables are taken from it) and the `*.CDM`
pictures. They are not included. The game looks for them:

- next to the executable and in its `Source` subdirectory;
- in the current directory and in its `Source` subdirectory;
- in the game's config directory (see below).

File names are case-insensitive. If the files are not found, the game opens
a window with a hint: drag the game folder (or the files themselves) into
that window, and the game copies them to the config directory. The
directory can also be given explicitly: `./cdman -data path/to/game`.

`HELP.CDM` and `HIGHSC.CDM` are optional: without the first there is no
help, the second holds the high score and statistics tables. `XMUSE.BIN`
from the distribution is not needed.

## Controls

| Key | Action |
|---|---|
| Arrows, numpad 8/4/6/2 | move; select in menus |
| First letter of a menu item | select the item |
| Enter | activate the menu item |
| Esc | in-game menu (Continue resumes) |
| Ctrl+Q | quit immediately, as in the original — high scores are not saved |
| F5 / F6 | decrease / increase video delay to match audio |
| F7 / F8 / F9 | slower / faster / normal reference machine speed |
| F11 or Alt+Enter | fullscreen |

In Double mode the first player uses W (up), A (left), D (right), X or Z
(down), the second player uses the arrows.

A gamepad works like the keyboard: D-pad and left stick are the arrows, the
bottom button and Start are Enter, the right button and Back are Esc. The
menu item “Control” → “Joystick” refers to the PC game port: the port has
none, so the game, like the original without a joystick, reports that no
joystick was found.

If the sound lags behind the picture (as happens with Bluetooth headphones
and speakers), adjust the delay with F5/F6: the step is 20 ms, the value is
shown in the window title and is remembered.

## Menu

The menu runs in text mode, as in the original.

- **SEE**: Top-10 — the ten best players (Space shows statistics), Info —
  help, Demo — demonstration, Position — look at the interrupted game.
- **SPEED**: game speed from Zzzzz… to Warp; the up and down arrows change
  it smoothly, the number shows the current value (63 is Normal).
- **GAME**: Start — new game, Continue — resume the interrupted one,
  Control — keyboard or joystick, Quit — exit and save the high scores.
- **PLAYER**: One — single player, Two — two players taking turns,
  Double — two players at once.
- **SOUND**: PC speaker sound on or off.

Closing the window is the same as Quit: playing time is counted and the
high scores are saved.

## Rules

CD-Man eats dots; when none are left, the stage is cleared. Monsters leave
the pen in the middle of the maze and chase CD-Man; the further the game
goes, the more eagerly they head straight for him. An energizer in a corner
(a fire in the park) makes the monsters edible for a while: they change
their look and run away, and an eaten monster (100, 200, 300 and 400 points
in a row) returns to the pen. From time to time a fruit appears in the maze
and is worth a random number of points. Some screens have tunnels. The park
by the castle has a key: with it CD-Man opens the gate, and a second key
gives points.

The game goes through the five screens twice, then a third round once
each. On the last screen of the third round the dots do not disappear and
the energizers come back: this stage lasts as long as the player has lives.
Whoever beats the best score in the table (with at least 10,000 points)
sees a special screen.

The high score table and statistics (playing time, stage, average speed,
number of moves, monsters and bonuses eaten) are stored in `HIGHSC.CDM` in
the same format as the original, so the file can be moved between the port
and the DOS version.

## Command-line flags

| Flag | Purpose |
|---|---|
| `-data path` | directory with the game files |
| `-speed 1.0` | reference machine speed (higher is faster) |
| `-fullscreen` | start in fullscreen |
| `-volume 0.7` | volume 0…1 |
| `-audio-delay ms` | video delay to match audio (defaults to the saved value) |

Debug flags are described in [docs/ORIGINAL.md](docs/ORIGINAL.md) (in
Russian).

## Config directory

The high score table (`HIGHSC.CDM`), settings (`settings.txt`) and game
files dropped into the window are kept in the config directory:

- Linux: `~/.config/cdman/`
- macOS: `~/Library/Application Support/cdman/`
- Windows: `%AppData%\cdman\`

On the first run the high score table is read from the game directory, and
from then on it is saved to the config directory.

## Differences from the original

- No copy protection: before the first game the original showed one of 16
  fires and asked for its name from the manual.
- Closing the window saves the high scores (in the original only Quit
  does).
- There is no game port joystick (see “Controls”).

Everything else follows the original, down to its bugs. The port was
checked against the original running in an emulator: with the same key
presses the game state matches byte for byte on every tick. Details are in
[docs/ORIGINAL.md](docs/ORIGINAL.md) (in Russian).

## Authors

The game was written by Anders Morée and Attila Biró, Creative Dimensions,
1992.

## License

The source code is distributed under the MIT license, see
[LICENSE](LICENSE).

The original game files are not part of the project. The rights to the
CD-MAN program, graphics and texts belong to their authors (© 1992 Creative
Dimensions); the MIT license does not apply to them. The text mode fonts are
based on [unscii-16](http://viznut.fi/unscii/), which is in the public
domain.
