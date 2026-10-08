package game

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func loadAllLevels(scenesDir string) ([]levelDef, error) {
	files := []string{"level.tscn", "level_2.tscn", "level_3.tscn"}
	levels := make([]levelDef, 0, len(files))
	for _, file := range files {
		lvl, err := loadLevelFromScene(filepath.Join(scenesDir, file))
		if err != nil {
			return nil, err
		}
		levels = append(levels, lvl)
	}
	return levels, nil
}

func loadLevelFromScene(path string) (levelDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return levelDef{}, err
	}
	text := string(data)
	lines := strings.Split(text, "\n")

	extMap := map[string]string{}
	extRe := regexp.MustCompile(`\[ext_resource[^\]]*path="([^"]+)"[^\]]*id="([^"]+)"\]`)
	for _, m := range extRe.FindAllStringSubmatch(text, -1) {
		extMap[m[2]] = m[1]
	}

	lvl := levelDef{
		Name:      filepath.Base(path),
		FilePath:  path,
		LevelTime: 60,
		MinTileX:  math.MaxInt,
		MinTileY:  math.MaxInt,
		MaxTileX:  math.MinInt,
		MaxTileY:  math.MinInt,
		Solids:    make(map[[2]int]struct{}),
	}

	if strings.Contains(text, "is_final_level = true") {
		lvl.IsFinal = true
	}

	levelTimeRe := regexp.MustCompile(`level_time\s*=\s*(\d+)`)
	if m := levelTimeRe.FindStringSubmatch(text); len(m) == 2 {
		if value, convErr := strconv.Atoi(m[1]); convErr == nil {
			lvl.LevelTime = value
		}
	}

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "layer_0/tile_data = PackedInt32Array(") {
			cells, parseErr := parseTileDataLine(trim)
			if parseErr != nil {
				return levelDef{}, fmt.Errorf("%s: %w", path, parseErr)
			}
			lvl.Tiles = cells
			for _, c := range cells {
				lvl.Solids[[2]int{c.X, c.Y}] = struct{}{}
				if c.X < lvl.MinTileX {
					lvl.MinTileX = c.X
				}
				if c.X > lvl.MaxTileX {
					lvl.MaxTileX = c.X
				}
				if c.Y < lvl.MinTileY {
					lvl.MinTileY = c.Y
				}
				if c.Y > lvl.MaxTileY {
					lvl.MaxTileY = c.Y
				}
			}
		}
	}

	var current *sceneNode

	positionRe := regexp.MustCompile(`position\s*=\s*Vector2\(([-0-9.]+),\s*([-0-9.]+)\)`)
	rotationRe := regexp.MustCompile(`rotation\s*=\s*([-0-9.]+)`)
	jumpForceRe := regexp.MustCompile(`jump_force\s*=\s*([-0-9.]+)`)

	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[node ") {
			if current != nil {
				applyNodeToLevel(lvl.FilePath, lvl.Name, &lvl, *current, extMap)
			}
			current = &sceneNode{}
			nameMatch := regexp.MustCompile(`name="([^"]+)"`).FindStringSubmatch(trim)
			if len(nameMatch) == 2 {
				current.Name = nameMatch[1]
			}
			if m := regexp.MustCompile(`instance=ExtResource\("([^"]+)"\)`).FindStringSubmatch(trim); len(m) == 2 {
				current.SceneRef = extMap[m[1]]
			}
			continue
		}
		if current == nil {
			continue
		}

		if m := positionRe.FindStringSubmatch(trim); len(m) == 3 {
			x, _ := strconv.ParseFloat(m[1], 64)
			y, _ := strconv.ParseFloat(m[2], 64)
			p := vec2{X: x, Y: y}
			current.Position = &p
		}
		if m := rotationRe.FindStringSubmatch(trim); len(m) == 2 {
			rot, _ := strconv.ParseFloat(m[1], 64)
			current.Rotation = rot
		}
		if m := jumpForceRe.FindStringSubmatch(trim); len(m) == 2 {
			force, _ := strconv.ParseFloat(m[1], 64)
			current.JumpForce = &force
		}
	}
	if current != nil {
		applyNodeToLevel(lvl.FilePath, lvl.Name, &lvl, *current, extMap)
	}

	if lvl.MinTileX == math.MaxInt {
		lvl.MinTileX = 0
		lvl.MaxTileX = virtualWidth / tileSize
		lvl.MinTileY = 0
		lvl.MaxTileY = virtualHeight / tileSize
	}
	lvl.WorldBottomY = float64((lvl.MaxTileY+1)*tileSize + 140)

	return lvl, nil
}

func applyNodeToLevel(_ string, _ string, lvl *levelDef, n sceneNode, _ map[string]string) {
	if n.Position == nil {
		return
	}
	scenePath := n.SceneRef
	sceneName := strings.ToLower(scenePath)

	switch {
	case n.Name == "Start" || strings.Contains(sceneName, "start.tscn"):
		lvl.StartPos = vec2{X: n.Position.X + 11, Y: n.Position.Y - 28}
	case n.Name == "Exit" || strings.Contains(sceneName, "exit.tscn"):
		lvl.ExitPos = vec2{X: n.Position.X, Y: n.Position.Y - 20}
	case strings.Contains(sceneName, "jump_pad.tscn"):
		force := 300.0
		if n.JumpForce != nil {
			force = *n.JumpForce
		}
		lvl.JumpPads = append(lvl.JumpPads, jumpPad{Pos: *n.Position, JumpForce: force})
	case strings.Contains(sceneName, "saw.tscn"):
		lvl.Traps = append(lvl.Traps, trap{Kind: trapSaw, Pos: *n.Position, Radius: 18})
	case strings.Contains(sceneName, "spike_ball.tscn"):
		lvl.Traps = append(lvl.Traps, trap{Kind: trapSpike, Pos: *n.Position, Radius: 11, Rotation: n.Rotation})
	}
}

func parseTileDataLine(line string) ([]tileCell, error) {
	start := strings.Index(line, "(")
	end := strings.LastIndex(line, ")")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("invalid tile_data line")
	}
	payload := line[start+1 : end]
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil, nil
	}

	parts := strings.Split(payload, ",")
	ints := make([]int, 0, len(parts))
	for _, raw := range parts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		v, err := strconv.Atoi(raw)
		if err != nil {
			return nil, err
		}
		ints = append(ints, v)
	}
	if len(ints)%3 != 0 {
		return nil, fmt.Errorf("tile_data entries are not triplets")
	}

	cells := make([]tileCell, 0, len(ints)/3)
	for i := 0; i < len(ints); i += 3 {
		packed := ints[i]
		source := ints[i+1]
		atlasY := ints[i+2]

		x := decodeSigned16(packed & 0xFFFF)
		y := decodeSigned16((packed >> 16) & 0xFFFF)
		atlasX := source >> 16

		cells = append(cells, tileCell{
			X:      x,
			Y:      y,
			AtlasX: atlasX,
			AtlasY: atlasY,
		})
	}
	return cells, nil
}

func decodeSigned16(v int) int {
	if v >= 0x8000 {
		return v - 0x10000
	}
	return v
}
