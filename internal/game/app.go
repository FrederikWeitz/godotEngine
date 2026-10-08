package game

import (
	"bytes"
	"fmt"
	"image"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

const (
	tileSize        = 16
	virtualWidth    = 480
	virtualHeight   = 270
	windowWidth     = 1440
	windowHeight    = 810
	cameraBottomY   = 255
	cameraStartY    = -40
	playerWidth     = 14.0
	playerHeight    = 23.0
	gravity         = 400.0
	maxFallSpeed    = 500.0
	playerSpeed     = 125.0
	playerJump      = 200.0
	exitDelaySec    = 1.5
	bgScrollSpeed   = 15.0
	spikeMoveRange  = 100.0
	spikePeriod     = 2.0
	audioSampleRate = 44100
)

type sceneState int

const (
	sceneMenu sceneState = iota
	scenePlaying
	sceneWin
)

type vec2 struct {
	X float64
	Y float64
}

type rect struct {
	X float64
	Y float64
	W float64
	H float64
}

func (r rect) Intersects(other rect) bool {
	return r.X < other.X+other.W && r.X+r.W > other.X && r.Y < other.Y+other.H && r.Y+r.H > other.Y
}

type tileCell struct {
	X      int
	Y      int
	AtlasX int
	AtlasY int
}

type jumpPad struct {
	Pos       vec2
	JumpForce float64
	AnimTime  float64
	Touching  bool
}

type trapType int

const (
	trapSaw trapType = iota
	trapSpike
)

type trap struct {
	Kind      trapType
	Pos       vec2
	Rotation  float64
	Radius    float64
	MoveTimer float64
}

type levelDef struct {
	Name          string
	FilePath      string
	Background    string
	LevelTime     int
	IsFinal       bool
	StartPos      vec2
	ExitPos       vec2
	Tiles         []tileCell
	Solids        map[[2]int]struct{}
	JumpPads      []jumpPad
	Traps         []trap
	MinTileX      int
	MaxTileX      int
	MinTileY      int
	MaxTileY      int
	WorldBottomY  float64
	ExitTriggered bool
	ExitTimer     float64
}

type sceneNode struct {
	Name      string
	SceneRef  string
	Position  *vec2
	Rotation  float64
	JumpForce *float64
}

type player struct {
	Pos     vec2
	Vel     vec2
	Active  bool
	FlipH   bool
	OnFloor bool

	Anim      string
	AnimTimer float64
}

func (p *player) Bounds() rect {
	return rect{
		X: p.Pos.X - playerWidth/2,
		Y: p.Pos.Y - playerHeight,
		W: playerWidth,
		H: playerHeight,
	}
}

type spriteStrip struct {
	Image      *ebiten.Image
	FrameW     int
	FrameH     int
	FrameCount int
}

type assets struct {
	terrain  *ebiten.Image
	bgBlue   *ebiten.Image
	bgGreen  *ebiten.Image
	bgPurple *ebiten.Image
	bgBrown  *ebiten.Image

	playerIdle  spriteStrip
	playerRun   spriteStrip
	playerJump  *ebiten.Image
	playerFall  *ebiten.Image
	jumpPadIdle *ebiten.Image
	jumpPadJump spriteStrip
	sawStrip    spriteStrip
	spikeBall   *ebiten.Image
	startStrip  spriteStrip
	exitStrip   spriteStrip

	jumpSFX   []byte
	hurtSFX   []byte
	musicLoop []byte
}

type game struct {
	state       sceneState
	audioCtx    *audio.Context
	musicPlayer *audio.Player

	levels       []levelDef
	levelIndex   int
	currentLevel *levelDef

	player      player
	timeLeft    int
	secondAccum float64

	camera   vec2
	bgOffset vec2

	assets assets
}

func Run() error {
	levels, err := loadAllLevels("data/godot/scenes")
	if err != nil {
		return fmt.Errorf("failed to load levels: %w", err)
	}
	if len(levels) == 0 {
		return fmt.Errorf("no levels loaded")
	}

	gameAssets, err := loadAssets("assets")
	if err != nil {
		return fmt.Errorf("failed to load assets: %w", err)
	}

	g := &game{
		state:    sceneMenu,
		levels:   levels,
		assets:   gameAssets,
		audioCtx: audio.NewContext(audioSampleRate),
	}
	if err := g.startMusicLoop(); err != nil {
		log.Printf("audio disabled: %v", err)
	}

	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("Martian Mike (Go)")

	if err := ebiten.RunGame(g); err != nil {
		return err
	}
	return nil
}

func (g *game) Layout(_, _ int) (int, int) {
	return virtualWidth, virtualHeight
}

func (g *game) Update() error {
	dt := 1.0 / 60.0

	switch g.state {
	case sceneMenu:
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.startGame()
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyQ) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			return ebiten.Termination
		}
	case sceneWin:
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			g.state = sceneMenu
		}
	case scenePlaying:
		if g.currentLevel == nil {
			return nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			return ebiten.Termination
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			g.reloadLevel()
			return nil
		}

		if !g.currentLevel.ExitTriggered {
			g.secondAccum += dt
			for g.secondAccum >= 1 {
				g.secondAccum -= 1
				g.timeLeft--
				if g.timeLeft < 0 {
					g.playHurtSFX()
					g.resetPlayer()
					g.timeLeft = g.currentLevel.LevelTime
				}
			}
		}

		g.bgOffset.X += bgScrollSpeed * dt
		g.bgOffset.Y += bgScrollSpeed * dt
		if g.bgOffset.X >= 64 {
			g.bgOffset.X = 0
		}
		if g.bgOffset.Y >= 64 {
			g.bgOffset.Y = 0
		}

		g.updatePlayer(dt)
		g.updateJumpPads(dt)
		g.updateTraps(dt)
		g.checkHazards()
		g.checkExit(dt)
		g.updateCamera()
	}

	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	switch g.state {
	case sceneMenu:
		g.drawBackground(screen, g.assets.bgGreen)
		ebitenutil.DebugPrintAt(screen, "MARTIAN MIKE", 180, 90)
		ebitenutil.DebugPrintAt(screen, "ENTER/SPACE: START", 160, 130)
		ebitenutil.DebugPrintAt(screen, "Q/ESC: QUIT", 175, 150)
		ebitenutil.DebugPrintAt(screen, "A/D: MOVE  SPACE: JUMP  R: RESET", 115, 220)
	case sceneWin:
		g.drawBackground(screen, g.assets.bgBrown)
		ebitenutil.DebugPrintAt(screen, "YOU WIN", 210, 110)
		ebitenutil.DebugPrintAt(screen, "ENTER/SPACE: OK", 180, 140)
	case scenePlaying:
		if g.currentLevel == nil {
			return
		}
		g.drawBackground(screen, g.resolveBackground())
		g.drawLevel(screen)
		g.drawHUD(screen)
	}
}

func (g *game) startGame() {
	g.levelIndex = 0
	g.state = scenePlaying
	g.loadLevel(0)
}

func (g *game) loadLevel(index int) {
	if index < 0 || index >= len(g.levels) {
		return
	}

	g.levelIndex = index
	g.currentLevel = &g.levels[g.levelIndex]
	g.currentLevel.ExitTriggered = false
	g.currentLevel.ExitTimer = 0
	for i := range g.currentLevel.JumpPads {
		g.currentLevel.JumpPads[i].Touching = false
		g.currentLevel.JumpPads[i].AnimTime = 0
	}
	for i := range g.currentLevel.Traps {
		g.currentLevel.Traps[i].MoveTimer = 0
	}

	g.player = player{
		Pos:    g.currentLevel.StartPos,
		Vel:    vec2{},
		Active: true,
		Anim:   "idle",
	}
	g.timeLeft = g.currentLevel.LevelTime
	g.secondAccum = 0
	g.camera = vec2{Y: cameraStartY}
	g.bgOffset = vec2{}
	g.updateCamera()
}

func (g *game) reloadLevel() {
	g.loadLevel(g.levelIndex)
}

func (g *game) resetPlayer() {
	if g.currentLevel == nil {
		return
	}
	g.player.Pos = g.currentLevel.StartPos
	g.player.Vel = vec2{}
	g.player.Active = true
	g.currentLevel.ExitTriggered = false
	g.currentLevel.ExitTimer = 0
}

func (g *game) updatePlayer(dt float64) {
	if g.currentLevel == nil {
		return
	}

	direction := 0.0
	if g.player.Active {
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyW) || inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			g.playJumpSFX()
			g.player.Vel.Y = -playerJump
		}
		if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
			direction -= 1
		}
		if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
			direction += 1
		}
	}

	if direction != 0 {
		g.player.FlipH = direction < 0
	}

	g.player.Vel.X = direction * playerSpeed
	g.player.Vel.Y += gravity * dt
	if g.player.Vel.Y > maxFallSpeed {
		g.player.Vel.Y = maxFallSpeed
	}

	g.player.OnFloor = false
	g.movePlayerX(g.player.Vel.X * dt)
	g.movePlayerY(g.player.Vel.Y * dt)

	g.player.AnimTimer += dt
	if g.player.OnFloor {
		if direction == 0 {
			g.player.Anim = "idle"
		} else {
			g.player.Anim = "run"
		}
	} else if g.player.Vel.Y < 0 {
		g.player.Anim = "jump"
	} else {
		g.player.Anim = "fall"
	}
}

func (g *game) movePlayerX(dx float64) {
	if dx == 0 {
		return
	}

	g.player.Pos.X += dx
	b := g.player.Bounds()
	minTX := int(math.Floor(b.X / tileSize))
	maxTX := int(math.Floor((b.X + b.W - 0.001) / tileSize))
	minTY := int(math.Floor(b.Y / tileSize))
	maxTY := int(math.Floor((b.Y + b.H - 0.001) / tileSize))

	for ty := minTY; ty <= maxTY; ty++ {
		for tx := minTX; tx <= maxTX; tx++ {
			if !g.isSolid(tx, ty) {
				continue
			}
			tileRect := rect{X: float64(tx * tileSize), Y: float64(ty * tileSize), W: tileSize, H: tileSize}
			if !g.player.Bounds().Intersects(tileRect) {
				continue
			}
			if dx > 0 {
				g.player.Pos.X = tileRect.X - playerWidth/2
			} else {
				g.player.Pos.X = tileRect.X + tileRect.W + playerWidth/2
			}
			g.player.Vel.X = 0
			return
		}
	}
}

func (g *game) movePlayerY(dy float64) {
	if dy == 0 {
		return
	}

	g.player.Pos.Y += dy
	b := g.player.Bounds()
	minTX := int(math.Floor(b.X / tileSize))
	maxTX := int(math.Floor((b.X + b.W - 0.001) / tileSize))
	minTY := int(math.Floor(b.Y / tileSize))
	maxTY := int(math.Floor((b.Y + b.H - 0.001) / tileSize))

	for ty := minTY; ty <= maxTY; ty++ {
		for tx := minTX; tx <= maxTX; tx++ {
			if !g.isSolid(tx, ty) {
				continue
			}
			tileRect := rect{X: float64(tx * tileSize), Y: float64(ty * tileSize), W: tileSize, H: tileSize}
			if !g.player.Bounds().Intersects(tileRect) {
				continue
			}
			if dy > 0 {
				g.player.Pos.Y = tileRect.Y
				g.player.OnFloor = true
			} else {
				g.player.Pos.Y = tileRect.Y + tileRect.H + playerHeight
			}
			g.player.Vel.Y = 0
			return
		}
	}
}

func (g *game) updateJumpPads(dt float64) {
	if g.currentLevel == nil {
		return
	}

	pb := g.player.Bounds()
	for i := range g.currentLevel.JumpPads {
		jp := &g.currentLevel.JumpPads[i]
		if jp.AnimTime > 0 {
			jp.AnimTime -= dt
			if jp.AnimTime < 0 {
				jp.AnimTime = 0
			}
		}

		padRect := rect{X: jp.Pos.X - 10, Y: jp.Pos.Y - 12, W: 20, H: 2}
		touching := pb.Intersects(padRect)
		if touching && !jp.Touching {
			g.playJumpSFX()
			g.player.Vel.Y = -jp.JumpForce
			jp.AnimTime = 0.4
		}
		jp.Touching = touching
	}
}

func (g *game) updateTraps(dt float64) {
	if g.currentLevel == nil {
		return
	}
	for i := range g.currentLevel.Traps {
		tr := &g.currentLevel.Traps[i]
		if tr.Kind == trapSpike {
			tr.MoveTimer += dt
			if tr.MoveTimer >= spikePeriod {
				tr.MoveTimer -= spikePeriod
			}
		}
	}
}

func (g *game) checkHazards() {
	if g.currentLevel == nil {
		return
	}

	if g.player.Pos.Y > g.currentLevel.WorldBottomY {
		g.playHurtSFX()
		g.resetPlayer()
		return
	}

	pb := g.player.Bounds()
	for _, tr := range g.currentLevel.Traps {
		tx, ty := tr.Pos.X, tr.Pos.Y
		if tr.Kind == trapSpike {
			dir := 1.0
			if math.Abs(tr.Rotation-math.Pi) < 0.01 {
				dir = -1
			}
			phase := tr.MoveTimer / spikePeriod
			pingPong := 1 - math.Abs(1-2*phase)
			tx += dir * spikeMoveRange * pingPong
		}
		trRect := rect{X: tx - tr.Radius, Y: ty - tr.Radius, W: tr.Radius * 2, H: tr.Radius * 2}
		if pb.Intersects(trRect) {
			g.playHurtSFX()
			g.resetPlayer()
			return
		}
	}
}

func (g *game) checkExit(dt float64) {
	if g.currentLevel == nil {
		return
	}

	exitRect := rect{X: g.currentLevel.ExitPos.X - 6, Y: g.currentLevel.ExitPos.Y - 6, W: 12, H: 12}
	if !g.currentLevel.ExitTriggered && g.player.Bounds().Intersects(exitRect) {
		g.currentLevel.ExitTriggered = true
		g.currentLevel.ExitTimer = 0
		g.player.Active = false
	}

	if g.currentLevel.ExitTriggered {
		g.currentLevel.ExitTimer += dt
		if g.currentLevel.ExitTimer >= exitDelaySec {
			if g.currentLevel.IsFinal || g.levelIndex == len(g.levels)-1 {
				g.state = sceneWin
				g.currentLevel = nil
			} else {
				g.loadLevel(g.levelIndex + 1)
			}
		}
	}
}

func (g *game) updateCamera() {
	if g.currentLevel == nil {
		return
	}
	minX := float64(g.currentLevel.MinTileX * tileSize)
	maxX := float64((g.currentLevel.MaxTileX + 1) * tileSize)
	worldW := maxX - minX
	targetX := g.player.Pos.X - float64(virtualWidth)/2
	if worldW <= virtualWidth {
		targetX = minX - (float64(virtualWidth)-worldW)/2
	} else {
		if targetX < minX {
			targetX = minX
		}
		maxCamX := maxX - float64(virtualWidth)
		if targetX > maxCamX {
			targetX = maxCamX
		}
	}
	g.camera.X = targetX

	margin := float64(virtualHeight) / 4
	playerTop := g.player.Pos.Y - 32
	playerBottom := g.player.Pos.Y
	if playerTop-g.camera.Y < margin {
		g.camera.Y = playerTop - margin
	} else if playerBottom-g.camera.Y > float64(virtualHeight)-margin {
		g.camera.Y = playerBottom - (float64(virtualHeight) - margin)
	}
	maxCameraY := float64(cameraBottomY - virtualHeight)
	if g.camera.Y > maxCameraY {
		g.camera.Y = maxCameraY
	}
}

func (g *game) drawBackground(screen *ebiten.Image, img *ebiten.Image) {
	if img == nil {
		screen.Fill(image.Black)
		return
	}
	w, h := img.Size()
	if w == 0 || h == 0 {
		return
	}
	offX := int(g.bgOffset.X) % w
	offY := int(g.bgOffset.Y) % h

	for y := -offY; y < virtualHeight+h; y += h {
		for x := -offX; x < virtualWidth+w; x += w {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), float64(y))
			screen.DrawImage(img, op)
		}
	}
}

func (g *game) drawLevel(screen *ebiten.Image) {
	lvl := g.currentLevel

	minDrawX := int(math.Floor(g.camera.X/tileSize)) - 1
	maxDrawX := int(math.Ceil((g.camera.X+virtualWidth)/tileSize)) + 1
	minDrawY := int(math.Floor(g.camera.Y/tileSize)) - 1
	maxDrawY := int(math.Ceil((g.camera.Y+virtualHeight)/tileSize)) + 1

	for _, cell := range lvl.Tiles {
		if cell.X < minDrawX || cell.X > maxDrawX || cell.Y < minDrawY || cell.Y > maxDrawY {
			continue
		}
		sx := cell.AtlasX * tileSize
		sy := cell.AtlasY * tileSize
		if sx < 0 || sy < 0 {
			continue
		}
		if g.assets.terrain == nil {
			continue
		}
		tw, th := g.assets.terrain.Size()
		if sx+tileSize > tw || sy+tileSize > th {
			continue
		}
		sub := g.assets.terrain.SubImage(image.Rect(sx, sy, sx+tileSize, sy+tileSize)).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(cell.X*tileSize)-g.camera.X, float64(cell.Y*tileSize)-g.camera.Y)
		screen.DrawImage(sub, op)
	}

	g.drawStart(screen)
	g.drawExit(screen)
	g.drawJumpPads(screen)
	g.drawTraps(screen)
	g.drawPlayer(screen)
}

func (g *game) drawStart(screen *ebiten.Image) {
	if g.assets.startStrip.Image == nil || g.currentLevel == nil {
		return
	}
	strip := g.assets.startStrip
	frame := int(g.player.AnimTimer*20) % maxInt(1, strip.FrameCount)
	sub := strip.Image.SubImage(image.Rect(frame*strip.FrameW, 0, frame*strip.FrameW+strip.FrameW, strip.FrameH)).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(g.currentLevel.StartPos.X-11-32-g.camera.X, g.currentLevel.StartPos.Y+28-64-g.camera.Y)
	screen.DrawImage(sub, op)
}

func (g *game) drawExit(screen *ebiten.Image) {
	if g.assets.exitStrip.Image == nil || g.currentLevel == nil {
		return
	}
	strip := g.assets.exitStrip
	frame := 0
	if g.currentLevel.ExitTriggered {
		frame = int(g.currentLevel.ExitTimer*20) % maxInt(1, strip.FrameCount)
	}
	sub := strip.Image.SubImage(image.Rect(frame*strip.FrameW, 0, frame*strip.FrameW+strip.FrameW, strip.FrameH)).(*ebiten.Image)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(g.currentLevel.ExitPos.X-32-g.camera.X, g.currentLevel.ExitPos.Y+20-64-g.camera.Y)
	screen.DrawImage(sub, op)
}

func (g *game) drawJumpPads(screen *ebiten.Image) {
	for _, jp := range g.currentLevel.JumpPads {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(jp.Pos.X-14-g.camera.X, jp.Pos.Y-28-g.camera.Y)
		if jp.AnimTime > 0 && g.assets.jumpPadJump.Image != nil {
			strip := g.assets.jumpPadJump
			frame := int((0.4-jp.AnimTime)*20) % maxInt(1, strip.FrameCount)
			sub := strip.Image.SubImage(image.Rect(frame*strip.FrameW, 0, frame*strip.FrameW+strip.FrameW, strip.FrameH)).(*ebiten.Image)
			screen.DrawImage(sub, op)
		} else if g.assets.jumpPadIdle != nil {
			screen.DrawImage(g.assets.jumpPadIdle, op)
		}
	}
}

func (g *game) drawTraps(screen *ebiten.Image) {
	for _, tr := range g.currentLevel.Traps {
		if tr.Kind == trapSaw {
			if g.assets.sawStrip.Image == nil {
				continue
			}
			strip := g.assets.sawStrip
			frame := int(time.Now().UnixNano()/1e8) % maxInt(1, strip.FrameCount)
			sub := strip.Image.SubImage(image.Rect(frame*strip.FrameW, 0, frame*strip.FrameW+strip.FrameW, strip.FrameH)).(*ebiten.Image)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(tr.Pos.X-float64(strip.FrameW)/2-g.camera.X, tr.Pos.Y-float64(strip.FrameH)/2-g.camera.Y)
			screen.DrawImage(sub, op)
			continue
		}

		if g.assets.spikeBall == nil {
			continue
		}
		x := tr.Pos.X
		dir := 1.0
		if math.Abs(tr.Rotation-math.Pi) < 0.01 {
			dir = -1
		}
		phase := tr.MoveTimer / spikePeriod
		pingPong := 1 - math.Abs(1-2*phase)
		x += dir * spikeMoveRange * pingPong

		op := &ebiten.DrawImageOptions{}
		w, h := g.assets.spikeBall.Size()
		op.GeoM.Translate(x-float64(w)/2-g.camera.X, tr.Pos.Y-float64(h)/2-g.camera.Y)
		screen.DrawImage(g.assets.spikeBall, op)
	}
}

func (g *game) drawPlayer(screen *ebiten.Image) {
	var img *ebiten.Image
	frameW, frameH := 32, 32

	switch g.player.Anim {
	case "run":
		if g.assets.playerRun.Image != nil {
			strip := g.assets.playerRun
			frame := int(g.player.AnimTimer*20) % maxInt(1, strip.FrameCount)
			img = strip.Image.SubImage(image.Rect(frame*strip.FrameW, 0, frame*strip.FrameW+strip.FrameW, strip.FrameH)).(*ebiten.Image)
			frameW, frameH = strip.FrameW, strip.FrameH
		}
	case "jump":
		img = g.assets.playerJump
	case "fall":
		img = g.assets.playerFall
	default:
		if g.assets.playerIdle.Image != nil {
			strip := g.assets.playerIdle
			frame := int(g.player.AnimTimer*20) % maxInt(1, strip.FrameCount)
			img = strip.Image.SubImage(image.Rect(frame*strip.FrameW, 0, frame*strip.FrameW+strip.FrameW, strip.FrameH)).(*ebiten.Image)
			frameW, frameH = strip.FrameW, strip.FrameH
		}
	}

	if img == nil {
		return
	}
	op := &ebiten.DrawImageOptions{}
	if g.player.FlipH {
		op.GeoM.Scale(-1, 1)
		op.GeoM.Translate(float64(frameW), 0)
	}
	op.GeoM.Translate(g.player.Pos.X-float64(frameW)/2-g.camera.X, g.player.Pos.Y-float64(frameH)-g.camera.Y)
	screen.DrawImage(img, op)
}

func (g *game) drawHUD(screen *ebiten.Image) {
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("TIME: %d", g.timeLeft), 210, 8)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("LEVEL: %d", g.levelIndex+1), 8, 8)
	ebitenutil.DebugPrintAt(screen, "R: RESET  ESC: QUIT", 330, 8)
}

func (g *game) resolveBackground() *ebiten.Image {
	switch g.levelIndex {
	case 0:
		return g.assets.bgBlue
	case 1:
		return g.assets.bgPurple
	default:
		return g.assets.bgBrown
	}
}

func (g *game) isSolid(tx, ty int) bool {
	if g.currentLevel == nil {
		return false
	}
	_, ok := g.currentLevel.Solids[[2]int{tx, ty}]
	return ok
}

func (g *game) startMusicLoop() error {
	if g.audioCtx == nil || len(g.assets.musicLoop) == 0 {
		return nil
	}
	loop := audio.NewInfiniteLoop(bytes.NewReader(g.assets.musicLoop), int64(len(g.assets.musicLoop)))
	player, err := g.audioCtx.NewPlayer(loop)
	if err != nil {
		return err
	}
	player.SetVolume(0.5)
	player.Play()
	g.musicPlayer = player
	return nil
}

func (g *game) playSFX(data []byte, volume float64) {
	if g.audioCtx == nil || len(data) == 0 {
		return
	}
	player := g.audioCtx.NewPlayerFromBytes(data)
	player.SetVolume(volume)
	player.Play()
}

func (g *game) playJumpSFX() {
	g.playSFX(g.assets.jumpSFX, 0.7)
}

func (g *game) playHurtSFX() {
	g.playSFX(g.assets.hurtSFX, 0.8)
}
