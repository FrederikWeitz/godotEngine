package game

import "testing"

func TestVerticalCameraFollowsPlayerWithinQuarterMargins(t *testing.T) {
	game := &game{
		currentLevel: &levelDef{MinTileX: 0, MaxTileX: 100},
		camera:       vec2{Y: cameraStartY},
		player:       player{Pos: vec2{X: 240, Y: 160}},
	}

	game.updateCamera()
	if game.camera.Y != cameraStartY {
		t.Fatalf("camera moved inside safe zone: got %v", game.camera.Y)
	}

	game.player.Pos.Y = 50
	game.updateCamera()
	if want := 50.0 - 32 - float64(virtualHeight)/4; game.camera.Y != want {
		t.Fatalf("camera did not follow upward jump: got %v, want %v", game.camera.Y, want)
	}

	game.player.Pos.Y = 160
	game.updateCamera()
	if want := 160.0 - float64(virtualHeight)*3/4; game.camera.Y != want {
		t.Fatalf("camera did not follow descent: got %v, want %v", game.camera.Y, want)
	}
}

func TestVerticalCameraStopsAtWorldBottom(t *testing.T) {
	game := &game{
		currentLevel: &levelDef{MinTileX: 0, MaxTileX: 100},
		camera:       vec2{Y: cameraStartY},
		player:       player{Pos: vec2{X: 240, Y: 200}},
	}

	game.updateCamera()
	if want := float64(cameraBottomY - virtualHeight); game.camera.Y != want {
		t.Fatalf("camera exceeded lower limit: got %v, want %v", game.camera.Y, want)
	}

	game.player.Pos.Y = 400
	game.updateCamera()
	if want := float64(cameraBottomY - virtualHeight); game.camera.Y != want {
		t.Fatalf("camera followed player beyond world bottom: got %v, want %v", game.camera.Y, want)
	}
}
