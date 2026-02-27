package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"image"
	_ "image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

//go:embed assets/*
var assetsFS embed.FS

// Physical Row Order of the PNG files
var animalRowMap = map[string][]string{
	"Bird":  {"Idle", "Fly", "Death"},
	"Bunny": {"Idle", "Jump", "Hit", "Death"},
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
	// Dimensions
	w, h int
	// Movement fields
	posX, posY float64
	facingLeft bool
	floatSeed  float64 // Used for bird hover oscillation
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
		// 1. Handle Animation Timing
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

		// 2. Handle Horizontal Movement
		curAnim := p.enabledAnims[p.currentIdx]
		if curAnim == "Walk" || curAnim == "Jump" || curAnim == "Fly" {
			moveSpeed := 1.0
			if curAnim == "Jump" || curAnim == "Fly" {
				moveSpeed = 1.6
			}

			if p.facingLeft {
				p.posX -= moveSpeed
			} else {
				p.posX += moveSpeed
			}

			// Boundary Check (Bounce back)
			scaledSpriteW := float64(p.w) * g.cfg.ScalingFactor
			if p.posX <= 0 {
				p.posX = 0
				p.facingLeft = false
			} else if p.posX >= float64(g.winW)-scaledSpriteW {
				p.posX = float64(g.winW) - scaledSpriteW
				p.facingLeft = true
			}
		}

		// 3. Bird "Hover" effect
		if p.species == "Bird" {
			p.floatSeed += 0.05
			// Adds a small up/down oscillation to the random height
			p.posY += math.Sin(p.floatSeed) * 0.2
		}
	}

	// Window Dragging
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

	for _, p := range g.pets {
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

		rect := image.Rect(p.currentFrame*p.w, row*p.h, (p.currentFrame+1)*p.w, (row+1)*p.h)

		op := &ebiten.DrawImageOptions{}

		// 1. Flip
		if p.facingLeft {
			op.GeoM.Scale(-1, 1)
			op.GeoM.Translate(float64(p.w), 0)
		}

		// 2. Scale
		op.GeoM.Scale(g.cfg.ScalingFactor, g.cfg.ScalingFactor)

		// 3. Position
		op.GeoM.Translate(p.posX, p.posY)

		screen.DrawImage(p.spriteImg.SubImage(rect).(*ebiten.Image), op)
	}
}

func (g *Game) Layout(w, h int) (int, int) { return g.winW, g.winH }

func main() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go config.json")
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	var cfg Config
	json.Unmarshal(data, &cfg)

	bgB, _ := assetsFS.ReadFile("assets/background.png")
	img, _, _ := image.Decode(bytes.NewReader(bgB))
	bgImg := ebiten.NewImageFromImage(img)

	finalH := int(cfg.TargetHeight * cfg.ScalingFactor)
	bgScale := float64(finalH) / float64(img.Bounds().Dy())
	finalW := int(float64(img.Bounds().Dx()) * bgScale)

	spacing := float64(finalW) / float64(len(cfg.ActiveAnimals)+1)

	var pets []*Pet
	for i, entry := range cfg.ActiveAnimals {
		sB, err := assetsFS.ReadFile("assets/sprites/Mini" + entry.Name + ".png")
		if err != nil {
			log.Printf("Could not find sprite for %s", entry.Name)
			continue
		}
		sImg, _, _ := image.Decode(bytes.NewReader(sB))

		pWidth, pHeight := 32, 32
		if entry.Name == "Bird" {
			pWidth, pHeight = 16, 16
		}

		// Calculate Y Position
		var startY float64
		scaledH := float64(pHeight) * cfg.ScalingFactor
		if entry.Name == "Bird" {
			// Random height between top and roughly 20px above the ground
			minY := 5.0
			maxY := float64(finalH) - scaledH - 20.0
			if maxY <= minY {
				startY = minY
			} else {
				startY = minY + rand.Float64()*(maxY-minY)
			}
		} else {
			// Grounded
			startY = float64(finalH) - scaledH
		}

		chosenAnims := entry.Animations
		if len(chosenAnims) == 1 && chosenAnims[0] == "all" {
			chosenAnims = animalRowMap[entry.Name]
		}

		pets = append(pets, &Pet{
			species:      entry.Name,
			spriteImg:    ebiten.NewImageFromImage(sImg),
			enabledAnims: chosenAnims,
			w:            pWidth,
			h:            pHeight,
			posX:         (spacing * float64(i+1)) - (float64(pWidth)*cfg.ScalingFactor)/2,
			posY:         startY,
			floatSeed:    rand.Float64() * 10,
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
