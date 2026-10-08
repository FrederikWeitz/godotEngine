package game

import (
	"bufio"
	"image"
	_ "image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

func loadAssets(base string) (assets, error) {
	loadImage := func(p string) (*ebiten.Image, error) {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		decoded, _, err := image.Decode(bufio.NewReader(f))
		if err != nil {
			return nil, err
		}
		return ebiten.NewImageFromImage(decoded), nil
	}

	loadPCMFromWAV := func(p string) ([]byte, error) {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		stream, err := wav.DecodeWithSampleRate(audioSampleRate, f)
		if err != nil {
			return nil, err
		}
		pcm, err := io.ReadAll(stream)
		if err != nil {
			return nil, err
		}
		return pcm, nil
	}

	loadPCMFromOgg := func(p string) ([]byte, error) {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		stream, err := vorbis.DecodeWithSampleRate(audioSampleRate, f)
		if err != nil {
			return nil, err
		}
		pcm, err := io.ReadAll(stream)
		if err != nil {
			return nil, err
		}
		return pcm, nil
	}

	mustStrip := func(path string, frameW, frameH int) (spriteStrip, error) {
		img, err := loadImage(path)
		if err != nil {
			return spriteStrip{}, err
		}
		w, _ := img.Size()
		frameCount := 1
		if frameW > 0 {
			frameCount = maxInt(1, w/frameW)
		}
		return spriteStrip{Image: img, FrameW: frameW, FrameH: frameH, FrameCount: frameCount}, nil
	}

	terrain, err := loadImage(filepath.Join(base, "textures", "Terrain (16x16).png"))
	if err != nil {
		return assets{}, err
	}
	bgBlue, err := loadImage(filepath.Join(base, "textures", "bg", "Blue.png"))
	if err != nil {
		return assets{}, err
	}
	bgGreen, err := loadImage(filepath.Join(base, "textures", "bg", "Green.png"))
	if err != nil {
		return assets{}, err
	}
	bgPurple, err := loadImage(filepath.Join(base, "textures", "bg", "Purple.png"))
	if err != nil {
		return assets{}, err
	}
	bgBrown, err := loadImage(filepath.Join(base, "textures", "bg", "Brown.png"))
	if err != nil {
		return assets{}, err
	}

	playerIdle, err := mustStrip(filepath.Join(base, "textures", "player", "Idle (32x32).png"), 32, 32)
	if err != nil {
		return assets{}, err
	}
	playerRun, err := mustStrip(filepath.Join(base, "textures", "player", "Run (32x32).png"), 32, 32)
	if err != nil {
		return assets{}, err
	}
	playerJump, err := loadImage(filepath.Join(base, "textures", "player", "Jump (32x32).png"))
	if err != nil {
		return assets{}, err
	}
	playerFall, err := loadImage(filepath.Join(base, "textures", "player", "Fall (32x32).png"))
	if err != nil {
		return assets{}, err
	}

	jumpPadIdle, err := loadImage(filepath.Join(base, "textures", "other", "Idle.png"))
	if err != nil {
		return assets{}, err
	}
	jumpPadJump, err := mustStrip(filepath.Join(base, "textures", "other", "Jump (28x28).png"), 28, 28)
	if err != nil {
		return assets{}, err
	}
	sawStrip, err := mustStrip(filepath.Join(base, "textures", "other", "Saw.png"), 38, 38)
	if err != nil {
		return assets{}, err
	}
	spikeBall, err := loadImage(filepath.Join(base, "textures", "other", "Spiked Ball.png"))
	if err != nil {
		return assets{}, err
	}
	startStrip, err := mustStrip(filepath.Join(base, "textures", "other", "Start.png"), 64, 64)
	if err != nil {
		return assets{}, err
	}
	exitStrip, err := mustStrip(filepath.Join(base, "textures", "other", "End (Pressed) (64x64).png"), 64, 64)
	if err != nil {
		return assets{}, err
	}

	jumpSFX, err := loadPCMFromWAV(filepath.Join(base, "audio", "jump.wav"))
	if err != nil {
		return assets{}, err
	}
	hurtSFX, err := loadPCMFromWAV(filepath.Join(base, "audio", "hurt.wav"))
	if err != nil {
		return assets{}, err
	}
	musicLoop, err := loadPCMFromOgg(filepath.Join(base, "audio", "music.ogg"))
	if err != nil {
		return assets{}, err
	}

	return assets{
		terrain:     terrain,
		bgBlue:      bgBlue,
		bgGreen:     bgGreen,
		bgPurple:    bgPurple,
		bgBrown:     bgBrown,
		playerIdle:  playerIdle,
		playerRun:   playerRun,
		playerJump:  playerJump,
		playerFall:  playerFall,
		jumpPadIdle: jumpPadIdle,
		jumpPadJump: jumpPadJump,
		sawStrip:    sawStrip,
		spikeBall:   spikeBall,
		startStrip:  startStrip,
		exitStrip:   exitStrip,
		jumpSFX:     jumpSFX,
		hurtSFX:     hurtSFX,
		musicLoop:   musicLoop,
	}, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
