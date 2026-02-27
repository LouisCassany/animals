package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"image"
	_ "image/png"
	"log"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

//go:embed assets/*
var assetsFS embed.FS

const (
	SpriteWidth  = 32
	SpriteHeight = 32
)

// Physical Row Order of the PNG files
var animalRowMap = map[string][]string{
	"Bird":  {"Idle", "Fly", "Death"},
	"Bunny": {"Idle", "Walk/Jump", "Hit", "Death"},
	"Fox":   {"Idle", "Walk", "Jump", "Attack", "Hit", "Death"},
	"Wolf":  {"Idle", "Walk", "Jump", "Attack", "Attack2", "Howl", "Hit", "Death"},
	"Boar":  {"Idle", "Walk", "Jump", "Attack", "Hit", "Death"},
	"Deer":  {"Idle", "Walk", "Jump", "Attack", "Hit", "Death"},
	"Deer2": {"Idle", "Walk", "Jump", "Attack", "Attack2", "Hit", "Death"},
	"Bear":  {"Idle", "Walk", "Jump", "Attack", "Attack2", "Roar", "Hit", "Death"},
}

type AnimalEntry struct {
	Name       string   `json:"name"`
	Animations []string `json:"animations"`
}

type Config struct {
	ScalingFactor float64                   `json:"scaling_factor"`
	TargetHeight  float64                   `json:"target_height"`
	ActiveAnimals []AnimalEntry             `json:"active_animals"`
	Library       map[string]map[string]int `json:"library"`
}

type Pet struct {
	species      string
	spriteImg    *ebiten.Image
	enabledAnims []string
	currentIdx   int
	currentFrame int
	ticks        int
}

type Game struct {
	cfg          Config
	pets         []*Pet
	bgImg        *ebiten.Image
	bgScale      float64
	winW, winH   int
	isDragging   bool
	dragX, dragY int
}

func (g *Game) Update() error {
	for _, p := range g.pets {
		p.ticks++
		if p.ticks > 10 {
			p.ticks = 0
			p.currentFrame++
			animName := p.enabledAnims[p.currentIdx]
			maxFrames := g.cfg.Library[p.species][animName]
			if p.currentFrame >= maxFrames {
				p.currentFrame = 0
				p.currentIdx = (p.currentIdx + 1) % len(p.enabledAnims)
			}
		}
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		g.isDragging = true
		g.dragX, g.dragY = ebiten.CursorPosition()
	}
	if g.isDragging {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			wx, wy := ebiten.WindowPosition()
			mx, my := ebiten.CursorPosition()
			ebiten.SetWindowPosition(wx+mx-g.dragX, wy+my-g.dragY)
		} else {
			g.isDragging = false
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	bgOp := &ebiten.DrawImageOptions{}
	bgOp.GeoM.Scale(g.bgScale, g.bgScale)
	screen.DrawImage(g.bgImg, bgOp)

	spacing := float64(g.winW) / float64(len(g.pets)+1)
	for i, p := range g.pets {
		animName := p.enabledAnims[p.currentIdx]

		row := -1
		for r, name := range animalRowMap[p.species] {
			if name == animName {
				row = r
				break
			}
		}
		if row == -1 {
			continue
		}

		rect := image.Rect(p.currentFrame*SpriteWidth, row*SpriteHeight, (p.currentFrame+1)*SpriteWidth, (row+1)*SpriteHeight)

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(g.cfg.ScalingFactor, g.cfg.ScalingFactor)

		dx := (spacing * float64(i+1)) - (float64(SpriteWidth)*g.cfg.ScalingFactor)/2
		dy := float64(g.winH) - (float64(SpriteHeight) * g.cfg.ScalingFactor)

		op.GeoM.Translate(dx, dy)
		screen.DrawImage(p.spriteImg.SubImage(rect).(*ebiten.Image), op)
	}
}

func (g *Game) Layout(w, h int) (int, int) { return g.winW, g.winH }

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go config.json")
	}

	data, _ := os.ReadFile(os.Args[1])
	var cfg Config
	json.Unmarshal(data, &cfg)

	bgB, _ := assetsFS.ReadFile("assets/background.png")
	img, _, _ := image.Decode(bytes.NewReader(bgB))
	bgImg := ebiten.NewImageFromImage(img)

	finalH := int(cfg.TargetHeight * cfg.ScalingFactor)
	bgScale := float64(finalH) / float64(img.Bounds().Dy())
	finalW := int(float64(img.Bounds().Dx()) * bgScale)

	var pets []*Pet
	for _, entry := range cfg.ActiveAnimals {
		sB, err := assetsFS.ReadFile("assets/sprites/Mini" + entry.Name + ".png")
		if err != nil {
			continue
		}
		sImg, _, _ := image.Decode(bytes.NewReader(sB))

		// Handle "all" keyword
		chosenAnims := entry.Animations
		if len(chosenAnims) == 1 && chosenAnims[0] == "all" {
			chosenAnims = animalRowMap[entry.Name]
		}

		pets = append(pets, &Pet{
			species:      entry.Name,
			spriteImg:    ebiten.NewImageFromImage(sImg),
			enabledAnims: chosenAnims,
		})
	}

	game := &Game{
		cfg:     cfg,
		pets:    pets,
		bgImg:   bgImg,
		bgScale: bgScale,
		winW:    finalW,
		winH:    finalH,
	}

	ebiten.SetWindowSize(finalW, finalH)
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)

	sw, sh := ebiten.Monitor().Size()
	ebiten.SetWindowPosition((sw-finalW)/2, sh-finalH-60)

	opts := &ebiten.RunGameOptions{ScreenTransparent: true}
	if err := ebiten.RunGameWithOptions(game, opts); err != nil {
		log.Fatal(err)
	}
}
