# Martian Mike (Go)

Diese Umsetzung portiert das Godot-Projekt `martian-mike` nach Go mit Ebiten.

## Enthaltene Features

- Startmenü (`ENTER/SPACE` zum Starten, `Q/ESC` zum Beenden)
- 3 Level mit Original-Tilemap-Daten aus `data/godot/scenes/*.tscn`
- Spielerbewegung, Sprung, Gravitation und Kollisionen
- Jump Pads, Saws, Spike Balls, Exit-Logik, Respawn und Level-Timer
- Win-Screen nach dem letzten Level
- Parallax-ähnlicher scrollender Hintergrund
- Nutzung der Originalgrafiken aus `assets/`
- Nutzung der Original-Audiodateien aus `assets/audio` (`music.ogg`, `jump.wav`, `hurt.wav`)

## Voraussetzungen

- Aktuelles Go (empfohlen: neueste stabile Version)
- Grafikkartentreiber aktuell

## Start

```bash
go mod tidy
go run ./cmd/martian-mike
```

## Struktur

- `cmd/martian-mike`: Einstiegspunkt (`main`)
- `internal/game`: Spiel-Engine, Update/Render, Audio, Level-/Asset-Loader
- `assets`: Laufzeit-Assets (Audio, Fonts, Texturen)
- `data/godot`: Original-Szenen, Scripts und Projektdateien als Referenzdaten

## Steuerung

- `A/D` oder `←/→`: Laufen
- `SPACE`, `W` oder `↑`: Springen
- `R`: Level neu laden
- `ESC`: Spiel beenden

## Hinweis

Alle nutzbaren Daten aus `source-godot` wurden in die oben genannten Zielordner übertragen; der alte Ordner wird nicht
mehr benötigt.
