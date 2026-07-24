package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

var bulletCooldownFrames = 30

type BulletCooldown struct {
	frames int
}

var bulletCooldown = BulletCooldown{frames: 0}

func (bc *BulletCooldown) Update() {
	if bc.frames > 0 {
		bc.frames--
	}
}

func (bc *BulletCooldown) Ready() bool {
	return bc.frames == 0
}

func (bc *BulletCooldown) Reset() {
	bc.frames = bulletCooldownFrames
}

const (
	screenWidth           = 1920
	screenHeight          = 1080
	playerWidth           = 128
	playerHeight          = 64
	speed                 = 7
	enemySize             = 55
	spawnInterval         = 20 // Aumentado para spawnar menos inimigos
	playerOffsetX         = 5.0
	playerOffsetY         = 5.0
	playerCollisionWidth  = playerWidth - 10
	playerCollisionHeight = playerHeight - 10
	maxGameTime           = 120.0 // 2 minutos em segundos
	maxPowerLevel         = 7
)

type Bullet struct {
	x, y   float64
	speedY float64
	dx     float64 // novo campo para movimento lateral
}

type Game struct {
	playerX    float64
	playerY    float64
	player     *ebiten.Image
	enemies    []Enemy
	bullets    []Bullet
	frameCount int
	gameOver   bool
	startTime  time.Time
	score      int // Agora é inteiro
	destroyed  int
	powerLevel int
	explosions []Explosion
	powerUps   []PowerUp
	gameTime   float64 // Tempo decorrido
}

type Enemy struct {
	x, y   float64
	speedY float64
}

type Explosion struct {
	x, y     float64
	frame    int
	maxFrame int
}

type PowerUp struct {
	x, y   float64
	active bool
}

var (
	regularFont font.Face
    smallFont   font.Face
)

func init() {
    fontBytes, err := os.ReadFile("Roboto-Regular.ttf") // Certifique-se de ter esse arquivo na pasta do projeto
    if err != nil {
        log.Fatal(err)
    }
    ttf, err := opentype.Parse(fontBytes)
    if err != nil {
        log.Fatal(err)
    }
    regularFont, err = opentype.NewFace(ttf, &opentype.FaceOptions{
        Size:    48, // Aumente ou diminua para ajustar o tamanho do texto
        DPI:     72,
        Hinting: font.HintingFull,
    })
    if err != nil {
        log.Fatal(err)
    }
    smallFont, err = opentype.NewFace(ttf, &opentype.FaceOptions{
        Size:    24,
	})
	if err != nil {
		log.Fatal(err)
	}
}

func (g *Game) Update() error {
	if g.gameOver {
		if ebiten.IsKeyPressed(ebiten.KeyR) {
			g.restart()
		}
		return nil
	}

	g.gameTime = time.Since(g.startTime).Seconds()
	if g.gameTime >= maxGameTime {
		g.gameOver = true
		g.gameTime = maxGameTime
		return nil
	}

	// Movimento do player (corrigido parênteses)
	if (ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA)) && g.playerX > 0 {
		g.playerX -= speed
	}
	if (ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD)) && g.playerX < screenWidth-playerWidth {
		g.playerX += speed
	}
	if (ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW)) && g.playerY > 0 {
		g.playerY -= speed
	}
	if (ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS)) && g.playerY < screenHeight-playerHeight {
		g.playerY += speed
	}

	// Limite para o player não sair da tela
	if g.playerX < 0 {
		g.playerX = 0
	}
	if g.playerX > float64(screenWidth-playerWidth) {
		g.playerX = float64(screenWidth - playerWidth)
	}
	if g.playerY < 0 {
		g.playerY = 0
	}
	if g.playerY > float64(screenHeight-playerHeight) {
		g.playerY = float64(screenHeight - playerHeight)
	}

	// Atualiza cooldown do tiro
	bulletCooldown.Update()

// Disparo de bolinha ao pressionar Espaço (com cooldown)
    if ebiten.IsKeyPressed(ebiten.KeySpace) && bulletCooldown.Ready() {
		b := Bullet{
			x:      g.playerX + playerWidth/2 - 5,
			y:      g.playerY,
			speedY: 10,
		}
		g.bullets = append(g.bullets, b)
		bulletCooldown.Reset()
	}

	// Atualiza tiros
	activeBullets := g.bullets[:0]
	for _, b := range g.bullets {
		b.y -= b.speedY
		b.x += b.dx    // move na diagonal se dx != 0
		if b.y > -25 { // Corrigido para altura do tiro
			activeBullets = append(activeBullets, b)
		}
	}
	g.bullets = activeBullets

	// Spawn e movimento dos inimigos
	g.frameCount++
	expoSpawn := int(float64(spawnInterval) / (1 + 0.03*g.gameTime)) // Diminui o crescimento da dificuldade
	if expoSpawn < 5 {                                               // Nunca menor que 5 frames
		expoSpawn = 5
	}
	if g.frameCount%expoSpawn == 0 {
		newEnemy := Enemy{
			x:      float64(rand.Intn(screenWidth - enemySize)),
			y:      -float64(enemySize),
			speedY: 1.5 + rand.Float64()*1.5, // Inimigos mais lentos
		}
		g.enemies = append(g.enemies, newEnemy)
	}
	for i := range g.enemies {
		g.enemies[i].y += g.enemies[i].speedY
	}
	activeEnemies := g.enemies[:0]
	for _, e := range g.enemies {
		if e.y < screenHeight {
			activeEnemies = append(activeEnemies, e)
		}
	}
	g.enemies = activeEnemies

	// Colisão tiro-inimigo
	newBullets := g.bullets[:0]
	newEnemies := g.enemies[:0]
	for _, e := range g.enemies {
		hit := false
		for i, b := range g.bullets {
			if bulletHitsEnemy(b, e) {
				hit = true
				g.bullets[i].y = -100
				g.destroyed++
				// Sistema de pontuação: quanto mais tempo, mais pontos por asteroide
				// Exemplo: 10 + int(g.gameTime/10)*5 pontos por asteroide
				points := 10 + int(g.gameTime/10)*5
				g.score += points

				// Adiciona explosão
				g.explosions = append(g.explosions, Explosion{
					x:        e.x + enemySize/2,
					y:        e.y + enemySize/2,
					frame:    0,
					maxFrame: 15,
				})
				// 1/10 de chance de power-up
				if rand.Intn(10) == 0 {
					g.powerUps = append(g.powerUps, PowerUp{
						x:      e.x + enemySize/2 - 10,
						y:      e.y + enemySize/2 - 10,
						active: true,
					})
				}
				break
			}
		}
		if !hit {
			newEnemies = append(newEnemies, e)
		}
	}
	// Remove tiros que acertaram
	for _, b := range g.bullets {
		if b.y > -25 {
			newBullets = append(newBullets, b)
		}
	}
	g.enemies = newEnemies
	g.bullets = newBullets

	// Colisão player-inimigo
	for _, e := range g.enemies {
		if checkCollision(g.playerX, g.playerY, e.x, e.y) {
			g.gameOver = true
		}
	}

	// Atualiza explosões
	activeExplosions := g.explosions[:0]
	for _, exp := range g.explosions {
		exp.frame++
		if exp.frame < exp.maxFrame {
			activeExplosions = append(activeExplosions, exp)
		}
	}
	g.explosions = activeExplosions

	// Atualiza power-ups
	activePowerUps := g.powerUps[:0]
	for _, p := range g.powerUps {
		if p.active {
			p.y += 3 // velocidade de queda mais lenta
			if checkCollision(g.playerX, g.playerY, p.x, p.y) {
				p.active = false
				// Diminui cooldown em 2, mínimo 20 (ajuste para efeito mais sutil)
				if bulletCooldownFrames-2 >= 20 {
					bulletCooldownFrames -= 2
				} else {
					bulletCooldownFrames = 20
				}
				if g.powerLevel < maxPowerLevel {
					g.powerLevel++
				}
			}
			if p.active && p.y < screenHeight {
				activePowerUps = append(activePowerUps, p)
			}
		}
	}
	g.powerUps = activePowerUps

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(image.Black)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(g.playerX, g.playerY)
	screen.DrawImage(g.player, op)

	// Exibe o tempo restante e comandos
	tempoRestante := int(maxGameTime - g.gameTime)
	minutos := tempoRestante / 60
	segundos := tempoRestante % 60
	bgColor := color.RGBA{0, 0, 0, 160}
	ebitenutil.DrawRect(screen, 10, 30, 420, 280, bgColor)
	text.Draw(screen, fmt.Sprintf("Tempo: %02d:%02d", minutos, segundos), regularFont, 20, 80, color.White)
	text.Draw(screen, fmt.Sprintf("Pontuação: %d", g.score), regularFont, 20, 140, color.White)
	text.Draw(screen, fmt.Sprintf("Destruidos: %d", g.destroyed), regularFont, 20, 200, color.White)
	text.Draw(screen, fmt.Sprintf("Power: %d/%d", g.powerLevel, maxPowerLevel), regularFont, 20, 260, color.White)
	text.Draw(screen, "Espaço: Atirar | R: Reiniciar", regularFont, 20, 320, color.White)

	// Desenha inimigos
	for _, e := range g.enemies {
		ebitenutil.DrawRect(screen, e.x, e.y, enemySize, enemySize, image.White)
	}
	// Desenha tiros (retângulo azul)
	blue := color.RGBA{0, 0, 171, 255}
	for _, b := range g.bullets {
		ebitenutil.DrawRect(screen, b.x, b.y, 10, 25, blue)
	}
	// Desenha explosões
	for _, exp := range g.explosions {
		alpha := uint8(255 - exp.frame*255/exp.maxFrame)
		radius := float64(exp.frame) * float64(enemySize) / float64(exp.maxFrame)
		ebitenutil.DrawCircle(screen, exp.x, exp.y, radius, color.RGBA{255, 200, 0, alpha})
	}
	// Desenha power-ups (quadrados verdes)
	for _, p := range g.powerUps {
		if p.active {
			ebitenutil.DrawRect(screen, p.x, p.y, 20, 20, color.RGBA{0, 255, 0, 255}) // Corrigido para verde
		}
	}
	if g.gameOver {
		text.Draw(screen, "FIM DE JOGO!", regularFont, screenWidth/2-200, screenHeight/2-40, color.White)
		text.Draw(screen, fmt.Sprintf("Pontuação final: %d", g.score), regularFont, screenWidth/2-200, screenHeight/2+20, color.White)
		text.Draw(screen, fmt.Sprintf("Inimigos destruídos: %d", g.destroyed), regularFont, screenWidth/2-200, screenHeight/2+80, color.White)
		text.Draw(screen, "Pressione R para reiniciar", regularFont, screenWidth/2-200, screenHeight/2+140, color.White)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

var playerImage image.Image
var playerAlpha [][]bool

// Inicializa playerAlpha ao carregar a imagem do player
func loadPlayerAlpha(img image.Image) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	playerAlpha = make([][]bool, height)
	for y := 0; y < height; y++ {
		playerAlpha[y] = make([]bool, width)
		for x := 0; x < width; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			playerAlpha[y][x] = a > 128 // 50% opaco
		}
	}
}

func loadImage(path string) *ebiten.Image {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		log.Fatal(err)
	}
	loadPlayerAlpha(img)
	return ebiten.NewImageFromImage(img)
}

func checkCollision(px, py float64, ex, ey float64) bool {
	exInt, eyInt := int(ex), int(ey)

	for y := 0; y < len(playerAlpha); y++ {
		for x := 0; x < len(playerAlpha[y]); x++ {
			if !playerAlpha[y][x] {
				continue // Pula pixels transparentes
			}

			screenX := int(px) + x
			screenY := int(py) + y

			// Verifica se esse pixel colide com a área do inimigo
			if screenX >= exInt && screenX < exInt+enemySize &&
				screenY >= eyInt && screenY < eyInt+enemySize {
				return true
			}
		}
	}
	return false
}

func bulletHitsEnemy(b Bullet, e Enemy) bool {
	return b.x < e.x+enemySize &&
		b.x+10 > e.x &&
		b.y < e.y+enemySize &&
		b.y+25 > e.y // Corrigido para altura do tiro
}

func (g *Game) restart() {
	g.playerX = screenWidth / 2
	g.playerY = screenHeight - playerHeight - 10
	g.enemies = []Enemy{}
	g.bullets = []Bullet{}
	g.startTime = time.Now()
	g.score = 0
	g.gameOver = false
	g.destroyed = 0
	g.powerLevel = 0
	g.gameTime = 0
	bulletCooldownFrames = 30
}

func main() {
	rand.Seed(time.Now().UnixNano())
	game := &Game{
		playerX:    250,
		playerY:    400,
		player:     loadImage("player.png"),
		enemies:    []Enemy{},
		frameCount: 0,
		startTime:  time.Now(),
	}
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Movimento do Jogador")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
