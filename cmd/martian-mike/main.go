package main

import (
	"log"

	"godotEngine/internal/game"
)

func main() {
	if err := game.Run(); err != nil {
		log.Fatal(err)
	}
}
