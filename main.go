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

	// Animation State
	currentAnim  string
	currentFrame int
	ticks        int
	loopCount    int
	targetLoops  int // How many times to repeat the current animation

	// Physics
	w, h       int
	posX, posY float64
	facingLeft bool
	floatSeed  float64
	speed      float64
}

// decideNextState gives each animal its "personality"
func (p *Pet) decideNextState() {
	p.currentFrame = 0
	p.loopCount = 0

	// Weighted Random Selection
	roll := rand.Intn(100)

	switch p.species {
	case "Bird":
		if roll < 70 {
			p.currentAnim = "Fly"
		} else {
			p.currentAnim = "Idle"
		}
		p.targetLoops = rand.Intn(5) + 2
	case "Bunny":
		if roll < 40 {
			p.currentAnim = "Jump"
		} else {
			p.currentAnim = "Idle"
		}
		p.targetLoops = rand.Intn(3) + 1
	case "Wolf":
		if roll < 10 {
			p.currentAnim = "Howl"
		} else if roll < 60 {
			p.currentAnim = "Walk"
		} else {
			p.currentAnim = "Idle"
		}
		p.targetLoops = rand.Intn(2) + 1
	case "Bear":
		if roll < 15 {
			p.currentAnim = "Roar"
		} else if roll < 50 {
			p.currentAnim = "Walk"
		} else {
			p.currentAnim = "Idle"
		}
		p.targetLoops = 1
	default:
		// Default logic for Fox, Deer, Boar
		if roll < 60 {
			p.currentAnim = "Walk"
		} else {
			p.currentAnim = "Idle"
		}
		p.targetLoops = rand.Intn(3) + 1
	}

	// Ensure the chosen animation exists for this sprite
	found := false
	for _, a := range p.enabledAnims {
		if a == p.currentAnim {
			found = true
			break
		}
	}
	if !found {
		p.currentAnim = p.enabledAnims[0]
	}
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

		// 1. Animation Timing Logic
		animLimit := 10
		if p.currentAnim == "Jump" || p.currentAnim == "Fly" {
			animLimit = 7 // Faster animation for movement
		}

		if p.ticks > animLimit {
			p.ticks = 0
			p.currentFrame++

			maxFrames := g.cfg.Library[p.species][p.currentAnim]
			if p.currentFrame >= maxFrames {
				p.currentFrame = 0
				p.loopCount++

				// If we've finished our intended number of loops, pick something new
				if p.loopCount >= p.targetLoops {
					p.decideNextState()
				}
			}
		}

		// 2. Movement Logic
		moveAmount := 0.0
		if p.currentAnim == "Walk" || p.currentAnim == "Fly" {
			moveAmount = p.speed
		} else if p.currentAnim == "Jump" {
			moveAmount = p.speed * 1.5
		}

		if p.facingLeft {
			p.posX -= moveAmount
		} else {
			p.posX += moveAmount
		}

		// 3. Smart Boundary Check
		scaledW := float64(p.w) * g.cfg.ScalingFactor
		if p.posX < 0 {
			p.posX = 0
			p.facingLeft = false
			p.currentAnim = "Idle" // Stop and think
			p.targetLoops = 2
		} else if p.posX > float64(g.winW)-scaledW {
			p.posX = float64(g.winW) - scaledW
			p.facingLeft = true
			p.currentAnim = "Idle" // Stop and think
			p.targetLoops = 2
		}

		// 4. Special Effects
		if p.species == "Bird" {
			p.floatSeed += 0.05
			p.posY += math.Sin(p.floatSeed) * 0.3
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
		row := -1
		for r, name := range animalRowMap[p.species] {
			if name == p.currentAnim {
				row = r
				break
			}
		}
		if row == -1 {
			continue
		}

		rect := image.Rect(p.currentFrame*p.w, row*p.h, (p.currentFrame+1)*p.w, (row+1)*p.h)
		op := &ebiten.DrawImageOptions{}

		if p.facingLeft {
			op.GeoM.Scale(-1, 1)
			op.GeoM.Translate(float64(p.w), 0)
		}
		op.GeoM.Scale(g.cfg.ScalingFactor, g.cfg.ScalingFactor)
		op.GeoM.Translate(p.posX, p.posY)

		screen.DrawImage(p.spriteImg.SubImage(rect).(*ebiten.Image), op)
	}
}

func (g *Game) Layout(w, h int) (int, int) { return g.winW, g.winH }

func main() {
	rand.Seed(time.Now().UnixNano())

	data, err := os.ReadFile("config.json")
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

	var pets []*Pet
	for _, entry := range cfg.ActiveAnimals {
		sB, err := assetsFS.ReadFile("assets/sprites/Mini" + entry.Name + ".png")
		if err != nil {
			continue
		}
		sImg, _, _ := image.Decode(bytes.NewReader(sB))

		pW, pH := 32, 32
		if entry.Name == "Bird" {
			pW, pH = 16, 16
		}

		scaledH := float64(pH) * cfg.ScalingFactor
		startY := float64(finalH) - scaledH
		if entry.Name == "Bird" {
			startY -= 20 + rand.Float64()*30
		}

		chosenAnims := entry.Animations
		if len(chosenAnims) == 1 && chosenAnims[0] == "all" {
			chosenAnims = animalRowMap[entry.Name]
		}

		// Personality Speed
		speed := 0.7 + rand.Float64()*0.5
		if entry.Name == "Bear" {
			speed = 0.4
		}
		if entry.Name == "Fox" {
			speed = 1.2
		}

		p := &Pet{
			species:      entry.Name,
			spriteImg:    ebiten.NewImageFromImage(sImg),
			enabledAnims: chosenAnims,
			w:            pW,
			h:            pH,
			posX:         rand.Float64() * float64(finalW-50),
			posY:         startY,
			facingLeft:   rand.Intn(2) == 0,
			floatSeed:    rand.Float64() * 10,
			speed:        speed,
		}
		p.decideNextState() // Initialize first state
		pets = append(pets, p)
	}

	game := &Game{cfg: cfg, pets: pets, bgImg: bgImg, bgScale: bgScale, winW: finalW, winH: finalH}
	ebiten.SetWindowSize(finalW, finalH)
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)

	sw, sh := ebiten.Monitor().Size()
	ebiten.SetWindowPosition((sw-finalW)/2, sh-finalH-60)

	if err := ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{ScreenTransparent: true, InitUnfocused: false}); err != nil {
		log.Fatal(err)
	}
}
