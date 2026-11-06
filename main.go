package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

const (
	screenWidth  = 1920
	screenHeight = 1080

	playerWidth  = 112
	playerHeight = 112
	playerSpeed  = 8.0

	playerHitW = 40.0
	playerHitH = 40.0

	maxLives    = 3
	maxPower    = 5
	maxRapid    = 3
	maxGameTime = 150.0

	enemyMinSize = 40.0
	enemyMaxSize = 78.0

	invulnFrames = 75
	shieldFrames = 450

	bossTriggerTime = 60.0
	bossWarnFrames  = 120
	bossMaxHP       = 600
	finalBossMaxHP  = 1000
	bossTargetY     = 130.0

	fortressSlowFrames = 18
)

const (
	powerWeapon = iota
	powerRapid
	powerShield
	powerHeal
)

type GameState int

const (
	stateMenu GameState = iota
	statePlaying
	statePaused
	stateGameOver
)

type Rect struct {
	X, Y, W, H float64
}

func (r Rect) Contains(x, y int) bool {
	fx, fy := float64(x), float64(y)
	return fx >= r.X && fx <= r.X+r.W && fy >= r.Y && fy <= r.Y+r.H
}

type Ship struct {
	name        string
	trait       string
	tint        color.RGBA
	speedMul    float64
	lifeBonus   int
	cooldownMul float64
}

var ships = []Ship{
	{name: "INTERCEPTADOR", trait: "+30% velocidade", tint: color.RGBA{90, 180, 255, 255}, speedMul: 1.3, cooldownMul: 1.0},
	{name: "FORTALEZA", trait: "+1 vida", tint: color.RGBA{90, 255, 140, 255}, speedMul: 0.92, cooldownMul: 1.0, lifeBonus: 1},
	{name: "CAÇADOR", trait: "+30% cadência", tint: color.RGBA{255, 100, 90, 255}, speedMul: 1.0, cooldownMul: 0.72},
	{name: "VANGUARDA", trait: "Canhão pesado", tint: color.RGBA{255, 200, 80, 255}, speedMul: 0.85, cooldownMul: 1.15},
	{name: "ESPECTRO", trait: "Lasers cruzados", tint: color.RGBA{190, 120, 255, 255}, speedMul: 1.1, cooldownMul: 0.85},
	{name: "TROVÃO", trait: "Tiro em zigue-zague", tint: color.RGBA{120, 255, 230, 255}, speedMul: 1.0, cooldownMul: 0.9},
}

var (
	menuPlayRect     = Rect{800, 582, 320, 64}
	menuQuitRect     = Rect{800, 656, 320, 64}
	pauseResumeRect  = Rect{800, 470, 320, 64}
	pauseRestartRect = Rect{800, 550, 320, 64}
	pauseMenuRect    = Rect{800, 630, 320, 64}
	overAgainRect    = Rect{770, 610, 380, 64}
	overMenuRect     = Rect{800, 690, 320, 64}

	shipCardRects = []Rect{
		{530, 255, 260, 130},
		{830, 255, 260, 130},
		{1130, 255, 260, 130},
		{530, 400, 260, 130},
		{830, 400, 260, 130},
		{1130, 400, 260, 130},
	}
)

type Bullet struct {
	x, y   float64
	speedY float64
	dx     float64
	damage int
	w, h   float64
	style  int
	color  color.RGBA
	wave   float64
	phase  float64
	pierce int
	homing float64
}

type Enemy struct {
	x, y       float64
	size       float64
	speedY     float64
	driftX     float64
	hp         int
	kind       int
	shootTimer int
}

type Boss struct {
	x, y      float64
	w, h      float64
	hp, maxHP int
	vx        float64
	fireTimer int
	healTimer int
	volley    int
	hitFlash  int
	entering  bool
	final     bool
}

type EnemyBullet struct {
	x, y   float64
	vx, vy float64
	r      float64
}

type Explosion struct {
	x, y      float64
	frame     int
	maxFrame  int
	maxRadius float64
	r, g, b   uint8
}

type PowerUp struct {
	x, y float64
	kind int
}

type Star struct {
	x, y       float64
	speed      float64
	size       float64
	brightness uint8
}

type Game struct {
	playerX float64
	playerY float64

	shipImages   []*ebiten.Image
	selectedShip int

	enemies      []Enemy
	bullets      []Bullet
	explosions   []Explosion
	powerUps     []PowerUp
	stars        []Star
	enemyBullets []EnemyBullet
	boss         *Boss

	state           GameState
	tick            int
	frameCount      int
	score           int
	bestScore       int
	destroyed       int
	lives           int
	livesMax        int
	powerLevel      int
	rapidLevel      int
	shieldTimer     int
	invulnTimer     int
	slowTimer       int
	fireCooldown    int
	combo           int
	comboTimer      int
	bossScheduled   bool
	bossWarn        int
	bossFightFrames int
	bossStage       int

	won       bool
	startTime time.Time
	pauseTime time.Time
	gameTime  float64

	shake  int
	shakeX float64
	shakeY float64
}

var (
	titleFont   font.Face
	regularFont font.Face
	mediumFont  font.Face
	smallFont   font.Face
)

func makeFace(ttf *sfnt.Font, size float64) font.Face {
	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	return face
}

func init() {
	fontBytes, err := os.ReadFile("Roboto-Regular.ttf")
	if err != nil {
		log.Fatal(err)
	}
	ttf, err := opentype.Parse(fontBytes)
	if err != nil {
		log.Fatal(err)
	}
	titleFont = makeFace(ttf, 120)
	regularFont = makeFace(ttf, 48)
	mediumFont = makeFace(ttf, 32)
	smallFont = makeFace(ttf, 24)
}

func newGame() *Game {
	g := &Game{shipImages: loadShipImages("player.png")}
	g.resetState()
	g.state = stateMenu
	return g
}

func (g *Game) resetState() {
	ship := ships[g.selectedShip]
	g.livesMax = maxLives + ship.lifeBonus

	g.playerX = screenWidth/2 - playerWidth/2
	g.playerY = screenHeight - playerHeight - 20
	g.enemies = nil
	g.bullets = nil
	g.explosions = nil
	g.powerUps = nil
	g.enemyBullets = nil
	g.boss = nil
	g.bossScheduled = false
	g.bossWarn = 0
	g.bossFightFrames = 0
	g.bossStage = 0
	g.frameCount = 0
	g.score = 0
	g.destroyed = 0
	g.lives = g.livesMax
	g.powerLevel = 0
	g.rapidLevel = 0
	g.shieldTimer = 0
	g.invulnTimer = 0
	g.slowTimer = 0
	g.fireCooldown = 0
	g.combo = 0
	g.comboTimer = 0
	g.won = false
	g.gameTime = 0
	g.shake = 0
	g.shakeX = 0
	g.shakeY = 0
	g.startTime = time.Now()
	g.initStars()
}

func (g *Game) initStars() {
	g.stars = make([]Star, 160)
	for i := range g.stars {
		g.stars[i] = Star{
			x:          rand.Float64() * screenWidth,
			y:          rand.Float64() * screenHeight,
			speed:      0.6 + rand.Float64()*3.4,
			size:       1 + rand.Float64()*2.5,
			brightness: uint8(90 + rand.Intn(165)),
		}
	}
}

func (g *Game) startGame() {
	g.resetState()
	g.state = statePlaying
}

func (g *Game) goToMenu() {
	if g.score > g.bestScore {
		g.bestScore = g.score
	}
	g.state = stateMenu
}

func (g *Game) finishGame(won bool) {
	g.state = stateGameOver
	g.won = won
	g.shake = 0
	g.shakeX = 0
	g.shakeY = 0
	if g.score > g.bestScore {
		g.bestScore = g.score
	}
}

func (g *Game) clicked(r Rect) bool {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return false
	}
	mx, my := ebiten.CursorPosition()
	return r.Contains(mx, my)
}

func (g *Game) Update() error {
	g.tick++
	g.updateStars()

	switch g.state {
	case stateMenu:
		return g.updateMenu()
	case statePaused:
		return g.updatePaused()
	case stateGameOver:
		return g.updateGameOver()
	default:
		return g.updatePlaying()
	}
}

func (g *Game) updateMenu() error {
	for i, r := range shipCardRects {
		if g.clicked(r) {
			g.selectedShip = i
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		g.selectedShip = (g.selectedShip + len(ships) - 1) % len(ships)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		g.selectedShip = (g.selectedShip + 1) % len(ships)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		g.selectedShip = (g.selectedShip + 3) % len(ships)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.selectedShip = (g.selectedShip + 3) % len(ships)
	}
	numberKeys := []ebiten.Key{
		ebiten.Key1, ebiten.Key2, ebiten.Key3,
		ebiten.Key4, ebiten.Key5, ebiten.Key6,
	}
	for i, k := range numberKeys {
		if i < len(ships) && inpututil.IsKeyJustPressed(k) {
			g.selectedShip = i
		}
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		g.clicked(menuPlayRect) {
		g.startGame()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || g.clicked(menuQuitRect) {
		return ebiten.Termination
	}
	return nil
}

func (g *Game) updatePlaying() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyP) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.state = statePaused
		g.pauseTime = time.Now()
		return nil
	}

	if g.boss == nil && g.bossWarn == 0 {
		g.gameTime = time.Since(g.startTime).Seconds()
		if g.gameTime >= maxGameTime {
			g.gameTime = maxGameTime
			if g.bossStage != 1 {
				g.finishGame(true)
				return nil
			}
			g.bossScheduled = false
		}
	}

	if g.comboTimer > 0 {
		g.comboTimer--
		if g.comboTimer == 0 {
			g.combo = 0
		}
	}

	g.updateShake()
	g.movePlayer()
	g.updateTimers()
	g.handleShooting()
	g.updateBullets()
	g.updateBoss()
	if g.bossWarn > 0 || g.boss != nil {
		g.bossFightFrames++
	}
	g.updateEnemyBullets()
	g.spawnEnemies()
	g.updateEnemies()
	g.resolveBulletHits()
	g.resolvePlayerHits()
	g.resolveBossPlayerHit()
	g.resolveBossBulletHits()
	g.updateExplosions()
	g.updatePowerUps()

	return nil
}

func (g *Game) updatePaused() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyP) ||
		inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
		g.clicked(pauseResumeRect) {
		g.startTime = g.startTime.Add(time.Since(g.pauseTime))
		g.state = statePlaying
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) || g.clicked(pauseRestartRect) {
		g.startGame()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) || g.clicked(pauseMenuRect) {
		g.goToMenu()
		return nil
	}
	return nil
}

func (g *Game) updateGameOver() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsKeyJustPressed(ebiten.KeySpace) ||
		inpututil.IsKeyJustPressed(ebiten.KeyR) ||
		g.clicked(overAgainRect) {
		g.startGame()
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) ||
		inpututil.IsKeyJustPressed(ebiten.KeyM) ||
		g.clicked(overMenuRect) {
		g.goToMenu()
		return nil
	}
	return nil
}

func (g *Game) updateTimers() {
	if g.fireCooldown > 0 {
		g.fireCooldown--
	}
	if g.shieldTimer > 0 {
		g.shieldTimer--
	}
	if g.invulnTimer > 0 {
		g.invulnTimer--
	}
	if g.slowTimer > 0 {
		g.slowTimer--
	}
}

func (g *Game) movePlayer() {
	mx, my := 0.0, 0.0
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		mx--
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		mx++
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		my--
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		my++
	}
	if mx != 0 || my != 0 {
		length := math.Hypot(mx, my)
		speed := playerSpeed * ships[g.selectedShip].speedMul
		if g.selectedShip == 1 && g.slowTimer > 0 {
			speed *= 0.35
		}
		g.playerX += mx / length * speed
		g.playerY += my / length * speed
	}
	if g.playerX < 0 {
		g.playerX = 0
	}
	if g.playerX > screenWidth-playerWidth {
		g.playerX = screenWidth - playerWidth
	}
	if g.playerY < 0 {
		g.playerY = 0
	}
	if g.playerY > screenHeight-playerHeight {
		g.playerY = screenHeight - playerHeight
	}
}

type shotSpec struct {
	off float64
	dx  float64
}

type homingShot struct {
	off    float64
	dx     float64
	homing float64
}

func (g *Game) handleShooting() {
	if !ebiten.IsKeyPressed(ebiten.KeySpace) || g.fireCooldown > 0 {
		return
	}

	switch g.selectedShip {
	case 0:
		g.shootInterceptor()
	case 1:
		g.shootFortress()
	case 2:
		g.shootHunter()
	case 3:
		g.shootVanguard()
	case 4:
		g.shootSpecter()
	default:
		g.shootStorm()
	}

	cooldown := 12 - g.rapidLevel*2
	if cooldown < 6 {
		cooldown = 6
	}
	cooldown = int(float64(cooldown) * ships[g.selectedShip].cooldownMul)
	if cooldown < 4 {
		cooldown = 4
	}
	g.fireCooldown = cooldown
}

func (g *Game) shootInterceptor() {
	cx := g.playerX + playerWidth/2
	cy := g.playerY + playerHeight*0.15
	col := color.RGBA{90, 210, 255, 255}

	var shots []shotSpec
	switch g.powerLevel {
	case 0:
		shots = []shotSpec{{0, 0}}
	case 1:
		shots = []shotSpec{{-8, 0}, {8, 0}}
	case 2:
		shots = []shotSpec{{0, 0}, {-16, -0.8}, {16, 0.8}}
	case 3:
		shots = []shotSpec{{-8, 0}, {8, 0}, {-24, -1.2}, {24, 1.2}}
	case 4:
		shots = []shotSpec{{0, 0}, {-12, -0.6}, {12, 0.6}, {-28, -1.4}, {28, 1.4}}
	default:
		shots = []shotSpec{{0, 0}, {-14, -0.8}, {14, 0.8}, {-32, -1.6}, {32, 1.6}}
	}

	for _, s := range shots {
		g.addBullet(cx+s.off, cy, s.dx, 18.0, 6, 32, 0, col)
	}

	if g.powerLevel >= 5 {
		g.addBulletAdvanced(cx, cy-8, 0, 22.0, 8, 56, 2, 4, color.RGBA{170, 245, 255, 255}, 4, 0)
	}
}

func (g *Game) shootFortress() {
	cx := g.playerX + playerWidth/2
	cy := g.playerY + playerHeight*0.15
	col := color.RGBA{110, 255, 140, 255}
	g.slowTimer = fortressSlowFrames

	count := 3
	spread := 1.8
	speed := 10.0
	size := 16.0
	switch g.powerLevel {
	case 1:
		count, spread = 5, 1.3
	case 2:
		count, spread, speed = 5, 1.9, 10.5
	case 3:
		count, spread, speed, size = 7, 1.4, 10.5, 15
	case 4:
		count, spread, speed = 7, 1.9, 11.0
	case 5:
		count, spread, speed, size = 9, 1.5, 11.0, 14
	}

	for i := 0; i < count; i++ {
		t := float64(i) - float64(count-1)/2
		g.addBullet(cx+t*6, cy, t*spread, speed, size, size, 1, col)
	}

	if g.powerLevel >= 2 {
		orbCol := color.RGBA{70, 230, 120, 255}
		g.addWaveBullet(cx-32, cy+6, -1.1, 8.2, 18, 18, 1, 1, orbCol, 1.4)
		g.addWaveBullet(cx+32, cy+6, 1.1, 8.2, 18, 18, 1, 1, orbCol, 1.4)
	}

	if g.powerLevel >= 4 {
		coreCol := color.RGBA{50, 210, 105, 255}
		g.addBullet(cx-10, cy+10, 0, 8.0, 28, 28, 1, coreCol)
		g.addBullet(cx+10, cy+10, 0, 8.0, 28, 28, 1, coreCol)
	}

	if g.powerLevel >= 5 {
		g.addBulletAdvanced(cx, cy+16, 0, 6.5, 42, 42, 2, 6, color.RGBA{45, 190, 95, 255}, 1, 0)
	}
}

func (g *Game) shootHunter() {
	cx := g.playerX + playerWidth/2
	cy := g.playerY + playerHeight*0.15
	col := color.RGBA{255, 120, 80, 255}

	var shots []homingShot
	switch g.powerLevel {
	case 0:
		shots = []homingShot{{-9, 0, 0}, {9, 0, 0}}
	case 1:
		shots = []homingShot{{0, 0, 0}, {-12, 0, 0}, {12, 0, 0}}
	case 2:
		shots = []homingShot{{-8, 0, 0}, {8, 0, 0}, {-20, -2.4, 0}, {20, 2.4, 0}}
	case 3:
		shots = []homingShot{{0, 0, 0}, {-10, -0.8, 0}, {10, 0.8, 0}, {-24, -2.8, 0}, {24, 2.8, 0}}
	case 4:
		shots = []homingShot{{0, 0, 0.8}, {-9, 0, 1.2}, {9, 0, 1.2}, {-22, -1.8, 2.2}, {22, 1.8, 2.2}, {-36, -3.4, 3.0}, {36, 3.4, 3.0}}
	default:
		shots = []homingShot{{0, 0, 2.0}, {-8, 0, 2.0}, {8, 0, 2.0}, {-18, -1.2, 2.6}, {18, 1.2, 2.6}, {-30, -2.6, 3.2}, {30, 2.6, 3.2}, {-44, -4.2, 3.6}, {44, 4.2, 3.6}}
	}

	for _, s := range shots {
		g.addBulletAdvanced(cx+s.off, cy, s.dx, 15.5, 8, 22, 1, 2, col, 0, s.homing)
	}

	if g.powerLevel >= 5 {
		g.addBulletAdvanced(cx, cy-6, 0, 17.0, 16, 30, 2, 5, color.RGBA{255, 200, 150, 255}, 0, 2.5)
	}
}

func (g *Game) shootVanguard() {
	cx := g.playerX + playerWidth/2
	cy := g.playerY + playerHeight*0.15
	col := color.RGBA{255, 200, 80, 255}

	var shots []shotSpec
	switch g.powerLevel {
	case 0:
		shots = []shotSpec{{0, 0}}
	case 1:
		shots = []shotSpec{{-16, 0}, {16, 0}}
	case 2:
		shots = []shotSpec{{0, 0}, {-22, 0}, {22, 0}}
	case 3:
		shots = []shotSpec{{-15, 0}, {15, 0}}
	case 4:
		shots = []shotSpec{{-16, 0}, {16, 0}}
	default:
		shots = []shotSpec{{-18, -0.4}, {18, 0.4}}
	}

	for _, s := range shots {
		g.addBulletDamage(cx+s.off, cy, s.dx, 9.5, 18, 38, 2, 3, col)
	}

	shardCol := color.RGBA{255, 230, 140, 255}
	switch {
	case g.powerLevel == 3:
		g.addBulletDamage(cx, cy-4, 0, 12.0, 12, 28, 1, 5, shardCol)
		g.addBulletDamage(cx-32, cy+4, -0.9, 11.0, 10, 24, 1, 5, shardCol)
		g.addBulletDamage(cx+32, cy+4, 0.9, 11.0, 10, 24, 1, 5, shardCol)
	case g.powerLevel == 4:
		g.addBulletAdvanced(cx, cy-6, 0, 11.5, 20, 48, 2, 3, color.RGBA{255, 235, 160, 255}, 1, 0)
		g.addBulletDamage(cx-34, cy+4, -1.0, 11.0, 10, 24, 1, 5, shardCol)
		g.addBulletDamage(cx+34, cy+4, 1.0, 11.0, 10, 24, 1, 5, shardCol)
	case g.powerLevel >= 5:
		g.addBulletAdvanced(cx, cy-8, 0, 13.5, 16, 68, 3, 4, color.RGBA{255, 245, 190, 255}, 4, 0)
		g.addBulletDamage(cx-36, cy+4, -1.1, 11.5, 10, 24, 1, 5, shardCol)
		g.addBulletDamage(cx+36, cy+4, 1.1, 11.5, 10, 24, 1, 5, shardCol)
	}
}

func (g *Game) shootSpecter() {
	cx := g.playerX + playerWidth/2
	cy := g.playerY + playerHeight*0.15
	col := color.RGBA{200, 140, 255, 255}

	var shots []shotSpec
	switch g.powerLevel {
	case 0:
		shots = []shotSpec{{-22, 1.6}, {22, -1.6}}
	case 1:
		shots = []shotSpec{{0, 0}, {-26, 1.8}, {26, -1.8}}
	case 2:
		shots = []shotSpec{{0, 0}, {-18, 1.4}, {18, -1.4}, {-34, 2.2}, {34, -2.2}}
	case 3:
		shots = []shotSpec{{-10, 0.7}, {10, -0.7}, {-26, 1.8}, {26, -1.8}, {-42, 2.6}, {42, -2.6}}
	case 4:
		shots = []shotSpec{{0, 0}, {-17, 1.1}, {17, -1.1}, {-32, 2.0}, {32, -2.0}, {-46, 2.8}, {46, -2.8}}
	default:
		shots = []shotSpec{{0, 0}, {-14, 0.9}, {14, -0.9}, {-28, 1.8}, {28, -1.8}, {-40, 2.5}, {40, -2.5}, {-52, 3.1}, {52, -3.1}}
	}

	sweep := 0.0
	if g.powerLevel >= 4 {
		sweep = 0.55 * math.Sin(float64(g.tick)*0.045)
	}

	for _, s := range shots {
		dx := s.dx + sweep*s.off/40.0
		if g.powerLevel >= 5 && (s.off <= -40 || s.off >= 40) {
			g.addWaveBullet(cx+s.off, cy, dx, 16.5, 5, 28, 1, 0, col, 1.9)
		} else {
			g.addBullet(cx+s.off, cy, dx, 16.5, 5, 26, 0, col)
		}
	}
}

func (g *Game) shootStorm() {
	cx := g.playerX + playerWidth/2
	cy := g.playerY + playerHeight*0.15
	col := color.RGBA{120, 255, 230, 255}

	var shots []shotSpec
	waveAmt := 3.0
	switch g.powerLevel {
	case 0:
		shots = []shotSpec{{0, 0}}
	case 1:
		shots = []shotSpec{{-10, 0}, {10, 0}}
	case 2:
		shots = []shotSpec{{0, 0}, {-16, -0.8}, {16, 0.8}}
	case 3:
		shots = []shotSpec{{-10, 0}, {10, 0}, {-26, -1.4}, {26, 1.4}}
		waveAmt = 3.4
	case 4:
		shots = []shotSpec{{0, 0}, {-16, -0.8}, {16, 0.8}, {-32, -1.8}, {32, 1.8}}
		waveAmt = 3.8
	default:
		shots = []shotSpec{{0, 0}, {-14, -0.6}, {14, 0.6}, {-28, -1.4}, {28, 1.4}, {-40, -2.2}, {40, 2.2}}
		waveAmt = 4.2
	}

	for _, s := range shots {
		g.addWaveBullet(cx+s.off, cy, s.dx, 14.5, 7, 22, 1, 2, col, waveAmt)
	}

	if g.powerLevel >= 4 {
		orbCol := color.RGBA{90, 240, 255, 255}
		g.addWaveBullet(cx-34, cy+6, -0.8, 7.5, 18, 18, 1, 6, orbCol, 5.5)
		g.addWaveBullet(cx+34, cy+6, 0.8, 7.5, 18, 18, 1, 6, orbCol, 5.5)
	}

	if g.powerLevel >= 5 {
		g.addWaveBullet(cx, cy+12, 0, 6.5, 30, 30, 2, 6, color.RGBA{140, 255, 245, 255}, 6.5)
	}
}

func (g *Game) addBullet(x, y, dx, speedY, w, h float64, style int, col color.RGBA) {
	g.addBulletDamage(x, y, dx, speedY, w, h, 1, style, col)
}

func (g *Game) addBulletDamage(x, y, dx, speedY, w, h float64, damage, style int, col color.RGBA) {
	g.bullets = append(g.bullets, Bullet{
		x: x, y: y, dx: dx, speedY: speedY, damage: damage,
		w: w, h: h, style: style, color: col,
	})
}

func (g *Game) addWaveBullet(x, y, dx, speedY, w, h float64, damage, style int, col color.RGBA, wave float64) {
	g.addBulletDamage(x, y, dx, speedY, w, h, damage, style, col)
	last := &g.bullets[len(g.bullets)-1]
	last.wave = wave
	last.phase = rand.Float64() * math.Pi * 2
}

func (g *Game) addBulletAdvanced(x, y, dx, speedY, w, h float64, damage, style int, col color.RGBA, pierce int, homing float64) {
	g.addBulletDamage(x, y, dx, speedY, w, h, damage, style, col)
	last := &g.bullets[len(g.bullets)-1]
	last.pierce = pierce
	last.homing = homing
}

func (g *Game) updateBullets() {
	alive := g.bullets[:0]
	for _, b := range g.bullets {
		if b.homing > 0 {
			if tx, _, ok := g.nearestTarget(b.x, b.y); ok {
				desired := (tx - b.x) * 0.02
				if desired > b.homing {
					desired = b.homing
				}
				if desired < -b.homing {
					desired = -b.homing
				}
				b.dx = b.dx*0.9 + desired*0.1
			}
		}
		b.y -= b.speedY
		if b.wave != 0 {
			b.x += math.Sin(float64(g.tick)*0.22+b.phase) * b.wave
		}
		b.x += b.dx
		if b.y > -80 && b.x > -80 && b.x < screenWidth+80 {
			alive = append(alive, b)
		}
	}
	g.bullets = alive
}

func (g *Game) nearestTarget(x, y float64) (float64, float64, bool) {
	best := math.MaxFloat64
	var tx, ty float64
	found := false
	for _, e := range g.enemies {
		if e.hp <= 0 {
			continue
		}
		cx := e.x + e.size/2
		cy := e.y + e.size/2
		d := (cx-x)*(cx-x) + (cy-y)*(cy-y)
		if d < best {
			best = d
			tx, ty = cx, cy
			found = true
		}
	}
	if b := g.boss; b != nil && !b.entering {
		cx := b.x + b.w/2
		cy := b.y + b.h/2
		d := (cx-x)*(cx-x) + (cy-y)*(cy-y)
		if d < best {
			tx, ty = cx, cy
			found = true
		}
	}
	return tx, ty, found
}

func (g *Game) spawnEnemies() {
	g.frameCount++
	if g.bossWarn > 0 {
		return
	}
	if g.boss != nil {
		if g.boss.entering {
			return
		}
		hpFrac := float64(g.boss.hp) / float64(g.boss.maxHP)
		interval := 58 - int((1-hpFrac)*24)
		if g.boss.final {
			interval -= 8
		}
		if interval < 30 {
			interval = 30
		}
		if g.frameCount%interval == 0 {
			size := 30.0 + rand.Float64()*8
			speed := 2.8 + rand.Float64()*1.6
			if g.boss.final {
				speed += 0.8
			}
			g.enemies = append(g.enemies, Enemy{
				x:      rand.Float64() * (screenWidth - size),
				y:      -size,
				size:   size,
				speedY: speed,
				driftX: (rand.Float64()*2 - 1) * 1.8,
				hp:     1,
				kind:   0,
			})
		}
		return
	}
	interval := int(44 - g.gameTime*0.22)
	if interval < 15 {
		interval = 15
	}
	if g.frameCount%interval != 0 {
		return
	}

	var size float64
	var hp, kind int
	r := rand.Float64()
	switch {
	case r < 0.48:
		size = enemyMinSize
		hp = 1
		kind = 0
	case r < 0.68:
		size = 58
		hp = 2
		kind = 1
	case r < 0.79:
		size = enemyMaxSize
		hp = 5
		kind = 2
	case r < 0.92:
		size = 48
		hp = 2
		kind = 3
	default:
		size = 62
		hp = 4
		kind = 4
	}

	speed := 1.9 + rand.Float64()*1.7 + g.gameTime*0.016
	drift := (rand.Float64()*2 - 1) * (0.8 + g.gameTime*0.005)

	speedY := speed
	switch kind {
	case 1:
		speedY -= 0.1
	case 2:
		speedY -= 0.2
	case 3:
		speedY += 0.9
	case 4:
		speedY -= 0.5
	}

	shootTimer := 0
	switch kind {
	case 1:
		shootTimer = 100 + rand.Intn(70)
	case 2:
		shootTimer = 80 + rand.Intn(60)
	case 3:
		shootTimer = 60 + rand.Intn(40)
	case 4:
		shootTimer = 90 + rand.Intn(50)
	}

	g.enemies = append(g.enemies, Enemy{
		x:          rand.Float64() * (screenWidth - size),
		y:          -size,
		size:       size,
		speedY:     speedY,
		driftX:     drift,
		hp:         hp,
		kind:       kind,
		shootTimer: shootTimer,
	})
}

func (g *Game) updateEnemies() {
	alive := g.enemies[:0]
	for _, e := range g.enemies {
		e.y += e.speedY
		e.x += e.driftX
		if e.x < 0 {
			e.x = 0
			e.driftX = -e.driftX
		}
		if e.x > screenWidth-e.size {
			e.x = screenWidth - e.size
			e.driftX = -e.driftX
		}
		if e.y > screenHeight+e.size {
			g.combo = 0
			g.comboTimer = 0
			continue
		}
		if e.kind > 0 && g.gameTime > 12 && e.y > 40 && e.y < screenHeight*0.72 {
			e.shootTimer--
			if e.shootTimer <= 0 {
				g.enemyShoot(e)
				base := 150 - int(g.gameTime*0.55)
				switch e.kind {
				case 3:
					base -= 35
				case 4:
					base += 30
				}
				if base < 62 {
					base = 62
				}
				e.shootTimer = base + rand.Intn(50)
			}
		}
		alive = append(alive, e)
	}
	g.enemies = alive
}

func (g *Game) enemyShoot(e Enemy) {
	cx := e.x + e.size/2
	cy := e.y + e.size*0.7
	px := g.playerX + playerWidth/2
	py := g.playerY + playerHeight/2
	base := math.Atan2(py-cy, px-cx)
	speed := 4.2 + rand.Float64()*1.2 + g.gameTime*0.006

	switch e.kind {
	case 2:
		count := 3
		for i := 0; i < count; i++ {
			a := base + (float64(i)-float64(count-1)/2)*0.16
			g.enemyBullets = append(g.enemyBullets, EnemyBullet{
				x: cx, y: cy, vx: math.Cos(a) * speed, vy: math.Sin(a) * speed, r: 8,
			})
		}
	case 4:
		count := 8
		slow := speed * 0.75
		for i := 0; i < count; i++ {
			a := float64(i)/float64(count)*2*math.Pi + float64(g.tick)*0.02
			g.enemyBullets = append(g.enemyBullets, EnemyBullet{
				x: cx, y: cy, vx: math.Cos(a) * slow, vy: math.Sin(a) * slow, r: 7,
			})
		}
	case 3:
		fast := speed + 1.6
		g.enemyBullets = append(g.enemyBullets, EnemyBullet{
			x: cx, y: cy, vx: math.Cos(base) * fast, vy: math.Sin(base) * fast, r: 6,
		})
	default:
		g.enemyBullets = append(g.enemyBullets, EnemyBullet{
			x: cx, y: cy, vx: math.Cos(base) * speed, vy: math.Sin(base) * speed, r: 7,
		})
	}
}

func (g *Game) resolveBulletHits() {
	survivingBullets := g.bullets[:0]
	for _, b := range g.bullets {
		hit := false
		consumed := false
		pierceLeft := b.pierce
		for i := range g.enemies {
			e := &g.enemies[i]
			if e.hp <= 0 || !bulletHitsEnemy(b, *e) {
				continue
			}
			e.hp -= b.damage
			hit = true
			if e.hp <= 0 {
				g.destroyEnemy(*e)
			} else {
				g.addExplosion(b.x, b.y, 14, 8, 255, 240, 160)
			}
			if pierceLeft > 0 {
				pierceLeft--
				continue
			}
			consumed = true
			break
		}
		b.pierce = pierceLeft
		if !hit && g.boss != nil && !g.boss.entering && bulletHitsBoss(b, g.boss) {
			g.boss.hp -= b.damage
			g.boss.hitFlash = 3
			hit = true
			consumed = true
			b.pierce = 0
			g.addExplosion(b.x, b.y, 12, 6, 255, 200, 120)
			if g.boss.hp <= 0 {
				g.defeatBoss()
			}
		}
		if !consumed {
			survivingBullets = append(survivingBullets, b)
		}
	}
	g.bullets = survivingBullets

	alive := g.enemies[:0]
	for _, e := range g.enemies {
		if e.hp > 0 {
			alive = append(alive, e)
		}
	}
	g.enemies = alive
}

func (g *Game) resolvePlayerHits() {
	alive := g.enemies[:0]
	for _, e := range g.enemies {
		if e.hp <= 0 {
			continue
		}
		if !playerHitsEnemy(g.playerX, g.playerY, e) {
			alive = append(alive, e)
			continue
		}
		if g.shieldTimer > 0 {
			g.destroyEnemy(e)
			continue
		}
		if g.invulnTimer > 0 {
			alive = append(alive, e)
			continue
		}
		g.destroyEnemy(e)
		g.damagePlayer()
	}
	g.enemies = alive
}

func (g *Game) damagePlayer() {
	if g.shieldTimer > 0 || g.invulnTimer > 0 {
		return
	}
	g.lives--
	g.invulnTimer = invulnFrames
	g.shake = 18
	g.combo = 0
	g.comboTimer = 0
	g.addExplosion(g.playerX+playerWidth/2, g.playerY+playerHeight/2, 90, 26, 255, 80, 60)
	if g.lives <= 0 {
		g.lives = 0
		g.finishGame(false)
	}
}

func (g *Game) resolveBossPlayerHit() {
	b := g.boss
	if b == nil || b.entering {
		return
	}
	if rectsOverlap(b.x+30, b.y+20, b.w-60, b.h-30, g.playerX, g.playerY, playerWidth, playerHeight) {
		g.damagePlayer()
	}
}

func (g *Game) resolveBossBulletHits() {
	alive := g.enemyBullets[:0]
	for _, eb := range g.enemyBullets {
		if !circleHitsPlayer(eb, g.playerX, g.playerY) {
			alive = append(alive, eb)
			continue
		}
		if g.shieldTimer > 0 || g.invulnTimer > 0 {
			g.addExplosion(eb.x, eb.y, 16, 8, 120, 220, 255)
			continue
		}
		g.damagePlayer()
		g.addExplosion(eb.x, eb.y, 20, 10, 255, 120, 60)
	}
	g.enemyBullets = alive
}

func (g *Game) spawnBoss() {
	finalBoss := g.bossStage >= 1
	w, h := 300.0, 170.0
	hp := bossMaxHP
	vx := 2.5
	if finalBoss {
		w, h = 340.0, 190.0
		hp = finalBossMaxHP
		vx = 3.2
	}

	g.boss = &Boss{
		x:         screenWidth/2 - w/2,
		y:         -h,
		w:         w,
		h:         h,
		hp:        hp,
		maxHP:     hp,
		vx:        vx,
		fireTimer: 80,
		healTimer: 300,
		entering:  true,
		final:     finalBoss,
	}
	g.enemyBullets = nil
	if finalBoss {
		g.powerUps = append(g.powerUps, PowerUp{
			x:    screenWidth/2 - 12,
			y:    180,
			kind: powerHeal,
		})
	}
	g.shake = 12
}

func (g *Game) updateBoss() {
	if g.bossStage > 1 {
		return
	}
	if !g.bossScheduled {
		trigger := bossTriggerTime
		if g.bossStage == 1 {
			trigger = maxGameTime
		}
		if g.gameTime < trigger {
			return
		}
		g.bossScheduled = true
		g.bossWarn = bossWarnFrames
		g.bossFightFrames = 0
		g.enemies = nil
		return
	}

	if g.bossWarn > 0 {
		g.bossWarn--
		if g.bossWarn == 0 {
			g.spawnBoss()
		}
		return
	}

	b := g.boss
	if b == nil {
		return
	}
	if b.hitFlash > 0 {
		b.hitFlash--
	}

	if b.entering {
		b.y += 3
		if b.y >= bossTargetY {
			b.y = bossTargetY
			b.entering = false
		}
		return
	}

	hpFrac := float64(b.hp) / float64(b.maxHP)
	speed := 2.4 + (1.0-hpFrac)*3.4
	if b.final {
		speed += 1.0
	}
	if b.vx < 0 {
		b.vx = -speed
	} else {
		b.vx = speed
	}
	b.x += b.vx
	if b.x < 60 {
		b.x = 60
		b.vx = math.Abs(b.vx)
	}
	if b.x+b.w > screenWidth-60 {
		b.x = screenWidth - 60 - b.w
		b.vx = -math.Abs(b.vx)
	}
	amp := 12.0
	if b.final {
		amp = 20.0
	}
	b.y = bossTargetY + math.Sin(float64(g.tick)*0.03)*amp

	b.fireTimer--
	if b.fireTimer <= 0 {
		g.bossShoot()
	}

	b.healTimer--
	if b.healTimer <= 0 {
		kind := powerHeal
		if g.lives >= g.livesMax {
			kind = rand.Intn(3)
		}
		g.powerUps = append(g.powerUps, PowerUp{
			x:    b.x + b.w/2 - 12 + (rand.Float64()*2-1)*120,
			y:    b.y + b.h*0.5 - 12,
			kind: kind,
		})
		b.healTimer = 420 + rand.Intn(140)
		if b.final {
			b.healTimer = 320 + rand.Intn(100)
		}
	}
}

func (g *Game) bossShoot() {
	b := g.boss
	if b == nil {
		return
	}
	cx := b.x + b.w/2
	cy := b.y + b.h*0.75
	hpFrac := float64(b.hp) / float64(b.maxHP)

	speed := 5.8
	count := 3
	spread := 0.14
	if hpFrac < 0.7 {
		speed = 6.4
		count = 6
	}
	if hpFrac < 0.35 {
		speed = 7.0
		count = 8
	}
	if b.final {
		speed += 0.9
		count++
		spread = 0.12
	}

	b.volley++
	radialEvery := 3
	if hpFrac < 0.35 {
		radialEvery = 2
	}
	if b.final {
		radialEvery = 2
	}
	if b.volley%radialEvery == 0 {
		n := 18
		if b.final {
			n = 20
		}
		for i := 0; i < n; i++ {
			a := float64(i)/float64(n)*2*math.Pi + rand.Float64()*0.08
			g.enemyBullets = append(g.enemyBullets, EnemyBullet{
				x: cx, y: cy, vx: math.Cos(a) * speed * 0.9, vy: math.Sin(a) * speed * 0.9, r: 9,
			})
		}
	} else {
		px := g.playerX + playerWidth/2
		py := g.playerY + playerHeight/2
		base := math.Atan2(py-cy, px-cx)
		for i := 0; i < count; i++ {
			off := (float64(i) - float64(count-1)/2) * spread
			a := base + off
			g.enemyBullets = append(g.enemyBullets, EnemyBullet{
				x: cx, y: cy, vx: math.Cos(a) * speed, vy: math.Sin(a) * speed, r: 8,
			})
		}
	}

	b.fireTimer = 62
	if hpFrac < 0.7 {
		b.fireTimer = 46
	}
	if hpFrac < 0.35 {
		b.fireTimer = 32
	}
	if b.final {
		b.fireTimer -= 6
	}
	b.fireTimer += rand.Intn(12)
}

func (g *Game) updateEnemyBullets() {
	alive := g.enemyBullets[:0]
	for _, eb := range g.enemyBullets {
		eb.x += eb.vx
		eb.y += eb.vy
		if eb.x > -40 && eb.x < screenWidth+40 && eb.y > -40 && eb.y < screenHeight+40 {
			alive = append(alive, eb)
		}
	}
	g.enemyBullets = alive
}

func (g *Game) defeatBoss() {
	b := g.boss
	if b == nil {
		return
	}
	cx := b.x + b.w/2
	cy := b.y + b.h/2

	wasFinal := b.final
	if wasFinal {
		g.score += 2500
	} else {
		g.score += 1000
	}
	g.destroyed++

	for i := 0; i < 18; i++ {
		angle := rand.Float64() * math.Pi * 2
		dist := rand.Float64() * b.w * 0.7
		g.addExplosion(
			cx+math.Cos(angle)*dist,
			cy+math.Sin(angle)*dist*0.6,
			50+rand.Float64()*90,
			24+rand.Intn(20),
			255,
			uint8(120+rand.Intn(110)),
			40,
		)
	}
	g.shake = 32

	for i := 0; i < 3; i++ {
		g.powerUps = append(g.powerUps, PowerUp{
			x:    cx - 12 + (rand.Float64()*2-1)*90,
			y:    cy,
			kind: rand.Intn(3),
		})
	}

	fightFrames := g.bossFightFrames
	g.boss = nil
	g.enemyBullets = nil
	g.bossFightFrames = 0
	g.bossScheduled = false
	g.bossWarn = 0
	if wasFinal {
		g.bossStage = 2
	} else {
		g.bossStage = 1
		g.startTime = g.startTime.Add(time.Duration(fightFrames) * time.Second / time.Duration(ebiten.TPS()))
	}
}

func comboMultiplier(combo int) int {
	mult := 1 + combo/5
	if mult > 5 {
		mult = 5
	}
	return mult
}

func (g *Game) destroyEnemy(e Enemy) {
	points := 10
	switch e.kind {
	case 1:
		points = 25
	case 2:
		points = 60
	case 3:
		points = 35
	case 4:
		points = 80
	}
	points += int(g.gameTime / 20)

	g.combo++
	g.comboTimer = 150
	g.score += points * comboMultiplier(g.combo)

	g.destroyed++
	g.addExplosion(e.x+e.size/2, e.y+e.size/2, e.size*0.9, 22, 255, 170, 40)
	g.shake = maxInt(g.shake, 3+int(e.size/20))

	dropChance := 11
	if g.boss != nil {
		dropChance = 30
	}
	if rand.Intn(100) < dropChance {
		kind := rand.Intn(3)
		if g.boss != nil && rand.Intn(100) < 55 {
			kind = powerHeal
		}
		g.powerUps = append(g.powerUps, PowerUp{
			x:    e.x + e.size/2 - 12,
			y:    e.y + e.size/2 - 12,
			kind: kind,
		})
	}
}

func (g *Game) addExplosion(x, y, radius float64, frames int, r, gg, b uint8) {
	g.explosions = append(g.explosions, Explosion{
		x: x, y: y, frame: 0, maxFrame: frames, maxRadius: radius, r: r, g: gg, b: b,
	})
}

func (g *Game) updateExplosions() {
	alive := g.explosions[:0]
	for _, e := range g.explosions {
		e.frame++
		if e.frame < e.maxFrame {
			alive = append(alive, e)
		}
	}
	g.explosions = alive
}

func (g *Game) updatePowerUps() {
	alive := g.powerUps[:0]
	for _, p := range g.powerUps {
		p.y += 2.6
		if p.y > screenHeight+30 {
			continue
		}
		hitSize := 24.0
		if p.kind == powerHeal {
			hitSize = 42.0
		}
		if rectsOverlap(p.x+12-hitSize/2, p.y+12-hitSize/2, hitSize, hitSize, g.playerX, g.playerY, playerWidth, playerHeight) {
			g.applyPowerUp(p.kind)
			continue
		}
		alive = append(alive, p)
	}
	g.powerUps = alive
}

func (g *Game) applyPowerUp(kind int) {
	switch kind {
	case powerWeapon:
		if g.powerLevel < maxPower {
			g.powerLevel++
		} else {
			g.score += 150
		}
	case powerRapid:
		if g.rapidLevel < maxRapid {
			g.rapidLevel++
		} else {
			g.score += 150
		}
	case powerShield:
		g.shieldTimer = shieldFrames
	case powerHeal:
		if g.lives < g.livesMax {
			g.lives++
		} else {
			g.score += 200
		}
	}
	g.addExplosion(g.playerX+playerWidth/2, g.playerY+playerHeight/2, 40, 14, 120, 255, 200)
}

func (g *Game) updateStars() {
	for i := range g.stars {
		s := &g.stars[i]
		s.y += s.speed
		if s.y > screenHeight {
			s.y = -4
			s.x = rand.Float64() * screenWidth
		}
	}
}

func (g *Game) updateShake() {
	if g.shake > 0 {
		g.shake--
		amp := float64(g.shake) * 0.9
		g.shakeX = (rand.Float64()*2 - 1) * amp
		g.shakeY = (rand.Float64()*2 - 1) * amp
		return
	}
	g.shakeX = 0
	g.shakeY = 0
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(image.Black)
	g.drawBackground(screen)

	switch g.state {
	case stateMenu:
		g.drawMenu(screen)
	case statePaused:
		g.drawWorld(screen)
		g.drawHUD(screen)
		g.drawBossWarning(screen)
		g.drawPause(screen)
	case stateGameOver:
		g.drawWorld(screen)
		g.drawHUD(screen)
		g.drawGameOver(screen)
	default:
		g.drawWorld(screen)
		g.drawHUD(screen)
		g.drawBossWarning(screen)
	}
}

func (g *Game) drawBackground(screen *ebiten.Image) {
	if g.state == stateMenu || g.state == stateGameOver {
		ebitenutil.DrawCircle(screen, 1560, 980, 360, color.RGBA{14, 22, 44, 255})
		ebitenutil.DrawCircle(screen, 1480, 900, 120, color.RGBA{24, 36, 64, 255})
	}
	g.drawStars(screen)
}

func (g *Game) drawStars(screen *ebiten.Image) {
	for _, s := range g.stars {
		c := color.RGBA{s.brightness, s.brightness, s.brightness, 255}
		ebitenutil.DrawRect(screen, s.x, s.y, s.size, s.size, c)
	}
}

func (g *Game) drawWorld(screen *ebiten.Image) {
	sx, sy := g.shakeX, g.shakeY
	g.drawPowerUps(screen, sx, sy)
	g.drawBoss(screen, sx, sy)
	g.drawEnemies(screen, sx, sy)
	g.drawBullets(screen, sx, sy)
	g.drawEnemyBullets(screen, sx, sy)
	g.drawExplosions(screen, sx, sy)
	g.drawPlayer(screen, sx, sy)
}

func (g *Game) drawPlayer(screen *ebiten.Image, sx, sy float64) {
	cx := g.playerX + playerWidth/2 + sx
	flame := 12 + 6*math.Sin(float64(g.tick)*0.5)
	by := g.playerY + playerHeight - 10 + sy
	ebitenutil.DrawRect(screen, cx-7, by, 14, flame, color.RGBA{255, 140, 40, 190})
	ebitenutil.DrawRect(screen, cx-3, by, 6, flame*0.65, color.RGBA{255, 235, 150, 220})

	if g.shieldTimer > 0 {
		alpha := uint8(70 + 40*math.Sin(float64(g.tick)*0.1))
		ebitenutil.DrawCircle(screen, cx, g.playerY+playerHeight/2+sy, playerWidth*0.55, color.RGBA{90, 220, 255, alpha})
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(g.playerX+sx, g.playerY+sy)
	if g.invulnTimer > 0 && (g.invulnTimer/6)%2 == 0 {
		op.ColorScale.ScaleAlpha(0.35)
	}
	screen.DrawImage(g.shipImages[g.selectedShip], op)
}

func (g *Game) drawBoss(screen *ebiten.Image, sx, sy float64) {
	b := g.boss
	if b == nil {
		return
	}
	x, y := b.x+sx, b.y+sy

	wing := color.RGBA{70, 22, 96, 255}
	body := color.RGBA{110, 42, 140, 255}
	if b.final {
		wing = color.RGBA{120, 18, 40, 255}
		body = color.RGBA{190, 45, 65, 255}
	}
	if b.hitFlash > 0 {
		wing = color.RGBA{210, 200, 230, 255}
		body = color.RGBA{255, 255, 255, 255}
	}

	inner := color.RGBA{140, 60, 170, 255}
	coreCol := color.RGBA{0, 50, 210, 255}
	spike := color.RGBA{200, 80, 240, 255}
	if b.final {
		inner = color.RGBA{210, 70, 80, 255}
		coreCol = color.RGBA{0, 60, 70, 255}
		spike = color.RGBA{255, 120, 110, 255}
	}

	ebitenutil.DrawRect(screen, x, y+45, b.w, 70, wing)
	ebitenutil.DrawRect(screen, x+50, y+30, b.w-100, b.h-50, wing)
	ebitenutil.DrawRect(screen, x+70, y, b.w-140, b.h, body)
	ebitenutil.DrawRect(screen, x+92, y+16, b.w-184, b.h-32, inner)

	ebitenutil.DrawRect(screen, x+55, y+b.h-42, 42, 48, color.RGBA{45, 45, 65, 255})
	ebitenutil.DrawRect(screen, x+b.w-97, y+b.h-42, 42, 48, color.RGBA{45, 45, 65, 255})

	pulse := 0.5 + 0.5*math.Sin(float64(g.tick)*0.15)
	core := uint8(140 + 110*pulse)
	coreCol.R = core
	ebitenutil.DrawCircle(screen, x+b.w/2, y+b.h/2, 36, coreCol)
	ebitenutil.DrawCircle(screen, x+b.w/2, y+b.h/2, 16, color.RGBA{255, 245, 255, 255})

	ebitenutil.DrawCircle(screen, x+88, y+62, 10, color.RGBA{255, 70, 70, 255})
	ebitenutil.DrawCircle(screen, x+b.w-88, y+62, 10, color.RGBA{255, 70, 70, 255})

	ebitenutil.DrawRect(screen, x+b.w/2-3, y-14, 6, 18, spike)
}

func (g *Game) drawEnemyBullets(screen *ebiten.Image, sx, sy float64) {
	for _, eb := range g.enemyBullets {
		ebitenutil.DrawCircle(screen, eb.x+sx, eb.y+sy, eb.r+3, color.RGBA{255, 70, 50, 90})
		ebitenutil.DrawCircle(screen, eb.x+sx, eb.y+sy, eb.r, color.RGBA{255, 140, 80, 255})
		ebitenutil.DrawCircle(screen, eb.x+sx, eb.y+sy, eb.r*0.45, color.RGBA{255, 240, 210, 255})
	}
}

func (g *Game) drawEnemies(screen *ebiten.Image, sx, sy float64) {
	for _, e := range g.enemies {
		var c color.RGBA
		switch e.kind {
		case 0:
			c = color.RGBA{210, 220, 235, 255}
		case 1:
			c = color.RGBA{255, 170, 60, 255}
		case 2:
			c = color.RGBA{240, 70, 70, 255}
		case 3:
			c = color.RGBA{255, 80, 220, 255}
		default:
			c = color.RGBA{80, 255, 220, 255}
		}

		if e.kind == 4 {
			cx := e.x + sx + e.size/2
			cy := e.y + sy + e.size/2
			ebitenutil.DrawCircle(screen, cx, cy, e.size/2, c)
			ebitenutil.DrawCircle(screen, cx, cy, e.size*0.3, color.RGBA{c.R / 2, c.G / 2, c.B / 2, 255})
			ebitenutil.DrawCircle(screen, cx, cy, e.size*0.12, color.White)
			continue
		}

		ebitenutil.DrawRect(screen, e.x+sx, e.y+sy, e.size, e.size, c)
		inner := e.size * 0.55
		ic := color.RGBA{c.R / 2, c.G / 2, c.B / 2, 255}
		ebitenutil.DrawRect(screen, e.x+sx+(e.size-inner)/2, e.y+sy+(e.size-inner)/2, inner, inner, ic)
		if e.kind == 3 {
			ebitenutil.DrawRect(screen, e.x+sx+e.size/2-2, e.y+sy+e.size*0.15, 4, e.size*0.7, color.White)
		}
	}
}

func (g *Game) drawBullets(screen *ebiten.Image, sx, sy float64) {
	for _, b := range g.bullets {
		x, y := b.x+sx, b.y+sy
		glow := color.RGBA{b.color.R, b.color.G, b.color.B, 80}
		switch b.style {
		case 0:
			ebitenutil.DrawRect(screen, x-b.w/2-3, y-b.h/2, b.w+6, b.h, glow)
			ebitenutil.DrawRect(screen, x-b.w/2, y-b.h/2, b.w, b.h, b.color)
			ebitenutil.DrawRect(screen, x-1, y-b.h/2+5, 2, b.h-10, color.RGBA{235, 250, 255, 255})
		case 1:
			ebitenutil.DrawCircle(screen, x, y, b.w/2+4, glow)
			ebitenutil.DrawCircle(screen, x, y, b.w/2, b.color)
			ebitenutil.DrawCircle(screen, x-b.w/6, y-b.h/6, b.w/5, color.RGBA{235, 255, 240, 255})
		case 3:
			ebitenutil.DrawRect(screen, x-b.w/2-4, y-b.h/2-4, b.w+8, b.h+8, glow)
			ebitenutil.DrawRect(screen, x-b.w/2, y-b.h/2, b.w, b.h, b.color)
			ebitenutil.DrawRect(screen, x-b.w/4, y-b.h/2+6, b.w/2, b.h-12, color.RGBA{255, 245, 220, 255})
		case 4:
			ebitenutil.DrawRect(screen, x-b.w/2-4, y-b.h/2, b.w+8, b.h, glow)
			ebitenutil.DrawRect(screen, x-b.w/2, y-b.h/2, b.w, b.h, b.color)
			ebitenutil.DrawRect(screen, x-1.5, y-b.h/2-6, 3, b.h+12, color.RGBA{245, 255, 255, 255})
			ebitenutil.DrawCircle(screen, x, y-b.h/2-6, 3, color.RGBA{245, 255, 255, 255})
		case 5:
			ebitenutil.DrawRect(screen, x-b.w/2-3, y-b.h/2-3, b.w+6, b.h+6, glow)
			ebitenutil.DrawRect(screen, x-b.w/2, y-b.h/2, b.w, b.h, b.color)
			ebitenutil.DrawRect(screen, x-b.w/2-4, y-2, b.w+8, 4, b.color)
			ebitenutil.DrawRect(screen, x-2, y-b.h/2-4, 4, b.h+8, b.color)
			ebitenutil.DrawCircle(screen, x, y, 2.5, color.RGBA{255, 250, 220, 255})
		case 6:
			pulse := 3 * math.Sin(float64(g.tick)*0.35+b.phase)
			ebitenutil.DrawCircle(screen, x, y, b.w/2+5+pulse, glow)
			ebitenutil.DrawCircle(screen, x, y, b.w/2, b.color)
			ebitenutil.DrawCircle(screen, x, y, b.w/4, color.RGBA{235, 255, 250, 255})
			ebitenutil.DrawRect(screen, x-b.w/2-3, y-1, b.w+6, 2, glow)
		default:
			ebitenutil.DrawRect(screen, x-b.w/2-2, y-b.h/2, b.w+4, b.h, glow)
			ebitenutil.DrawRect(screen, x-b.w/2, y-b.h/2, b.w, b.h, b.color)
			ebitenutil.DrawRect(screen, x-1, y-b.h/2+4, 2, b.h-8, color.RGBA{255, 240, 210, 255})
		}
	}
}

func (g *Game) drawExplosions(screen *ebiten.Image, sx, sy float64) {
	for _, e := range g.explosions {
		t := float64(e.frame) / float64(e.maxFrame)
		alpha := uint8(255 * (1 - t))
		radius := e.maxRadius * t
		ebitenutil.DrawCircle(screen, e.x+sx, e.y+sy, radius, color.RGBA{e.r, e.g, e.b, alpha})
		if t < 0.6 {
			ebitenutil.DrawCircle(screen, e.x+sx, e.y+sy, radius*0.5, color.RGBA{255, 255, 255, uint8(255 * (0.6 - t))})
		}
	}
}

func (g *Game) drawPowerUps(screen *ebiten.Image, sx, sy float64) {
	for _, p := range g.powerUps {
		var c color.RGBA
		switch p.kind {
		case powerWeapon:
			c = color.RGBA{80, 255, 130, 255}
		case powerRapid:
			c = color.RGBA{255, 220, 70, 255}
		case powerShield:
			c = color.RGBA{90, 220, 255, 255}
		default:
			c = color.RGBA{255, 90, 120, 255}
		}

		cx := p.x + sx + 12
		cy := p.y + sy + 12
		box := 24.0
		if p.kind == powerHeal {
			box = 42.0
		}
		size := box*0.9 + 3*math.Sin(float64(g.tick)*0.18)
		glow := color.RGBA{c.R, c.G, c.B, 90}
		ebitenutil.DrawRect(screen, cx-size/2-5, cy-size/2-5, size+10, size+10, glow)

		if p.kind == powerHeal {
			ebitenutil.DrawRect(screen, cx-size/2-2, cy-size/2-2, size+4, size+4, color.RGBA{255, 235, 240, 255})
		}

		ebitenutil.DrawRect(screen, cx-size/2, cy-size/2, size, size, c)

		if p.kind == powerHeal {
			arm := size * 0.62
			thick := size * 0.24
			ebitenutil.DrawRect(screen, cx-arm/2, cy-thick/2, arm, thick, color.White)
			ebitenutil.DrawRect(screen, cx-thick/2, cy-arm/2, thick, arm, color.White)
		} else {
			inner := size * 0.3
			ebitenutil.DrawRect(screen, cx-inner/2, cy-inner/2, inner, inner, color.White)
		}
	}
}

var (
	hudLabel = color.RGBA{130, 160, 210, 255}
	hudDim   = color.RGBA{95, 120, 160, 255}
)

func drawPanel(screen *ebiten.Image, r Rect, fill, border color.RGBA) {
	ebitenutil.DrawRect(screen, r.X, r.Y, r.W, r.H, fill)
	ebitenutil.DrawRect(screen, r.X, r.Y, r.W, 2, border)
}

func (g *Game) drawHUD(screen *ebiten.Image) {
	tempo := int(maxGameTime - g.gameTime)
	if tempo < 0 {
		tempo = 0
	}

	left := Rect{24, 24, 360, 136}
	drawPanel(screen, left, color.RGBA{6, 10, 24, 205}, color.RGBA{70, 110, 180, 190})
	text.Draw(screen, "TEMPO", smallFont, 44, 58, hudLabel)
	timeText := fmt.Sprintf("%02d:%02d", tempo/60, tempo%60)
	if g.bossStage == 1 && g.gameTime >= maxGameTime {
		timeText = "FINAL"
	}
	text.Draw(screen, timeText, smallFont, 200, 58, color.White)
	text.Draw(screen, "PONTOS", smallFont, 44, 98, hudLabel)
	text.Draw(screen, fmt.Sprintf("%d", g.score), smallFont, 200, 98, color.White)
	text.Draw(screen, "RECORDE", smallFont, 44, 138, hudLabel)
	text.Draw(screen, fmt.Sprintf("%d", g.bestScore), smallFont, 200, 138, color.White)

	right := Rect{1506, 24, 390, 196}
	drawPanel(screen, right, color.RGBA{6, 10, 24, 205}, color.RGBA{70, 110, 180, 190})

	text.Draw(screen, "VIDAS", smallFont, 1526, 58, hudLabel)
	for i := 0; i < g.lives; i++ {
		ebitenutil.DrawCircle(screen, float64(1660+i*30), 50, 9, color.RGBA{255, 90, 110, 255})
	}

	text.Draw(screen, "ARMA", smallFont, 1526, 100, hudLabel)
	for i := 0; i < maxPower; i++ {
		c := color.RGBA{40, 56, 88, 255}
		if i < g.powerLevel {
			c = color.RGBA{80, 255, 130, 255}
		}
		ebitenutil.DrawRect(screen, float64(1660+i*24), 84, 18, 18, c)
	}

	text.Draw(screen, "CADÊNCIA", smallFont, 1526, 142, hudLabel)
	for i := 0; i < maxRapid; i++ {
		c := color.RGBA{40, 56, 88, 255}
		if i < g.rapidLevel {
			c = color.RGBA{255, 220, 70, 255}
		}
		ebitenutil.DrawRect(screen, float64(1660+i*24), 126, 18, 18, c)
	}

	text.Draw(screen, "ESCUDO", smallFont, 1526, 184, hudLabel)
	ebitenutil.DrawRect(screen, 1660, 172, 190, 12, color.RGBA{40, 56, 88, 255})
	if g.shieldTimer > 0 {
		barW := 190.0 * float64(g.shieldTimer) / shieldFrames
		ebitenutil.DrawRect(screen, 1660, 172, barW, 12, color.RGBA{90, 220, 255, 255})
	}

	if g.boss != nil || g.bossWarn > 0 {
		g.drawBossBar(screen)
	}

	if g.combo >= 3 {
		label := fmt.Sprintf("COMBO %d   x%d", g.combo, comboMultiplier(g.combo))
		c := color.RGBA{255, 230, 90, 255}
		if g.combo >= 10 {
			c = color.RGBA{255, 120, 60, 255}
		}
		comboY := 72
		if g.boss != nil || g.bossWarn > 0 {
			comboY = 118
		}
		w := font.MeasureString(mediumFont, label).Round()
		text.Draw(screen, label, mediumFont, screenWidth/2-w/2, comboY, c)
	}

	hint := "WASD / SETAS  Mover     ESPAÇO  Atirar     P / ESC  Pausar"
	drawPanel(screen, Rect{24, screenHeight - 58, 690, 38}, color.RGBA{6, 10, 24, 160}, color.RGBA{0, 0, 0, 0})
	text.Draw(screen, hint, smallFont, 40, screenHeight-31, hudDim)
}

func (g *Game) drawBossBar(screen *ebiten.Image) {
	b := g.boss
	if b == nil {
		return
	}

	w := 900.0
	x := (screenWidth - w) / 2
	y := 44.0
	h := 26.0

	ebitenutil.DrawRect(screen, x-2, y-2, w+4, h+4, color.RGBA{170, 45, 70, 255})
	ebitenutil.DrawRect(screen, x, y, w, h, color.RGBA{28, 8, 14, 235})

	frac := float64(b.hp) / float64(b.maxHP)
	if frac < 0 {
		frac = 0
	}
	fill := color.RGBA{230, 40, 70, 255}
	if frac < 0.35 {
		fill = color.RGBA{255, 95, 40, 255}
	}
	ebitenutil.DrawRect(screen, x, y, w*frac, h, fill)
	ebitenutil.DrawRect(screen, x, y, w*frac, 6, color.RGBA{255, 150, 170, 255})

	label := "CHEFÃO"
	if b.final {
		label = "CHEFÃO FINAL"
	}
	text.Draw(screen, label, smallFont, int(x)+10, int(y)+20, color.White)
	pct := fmt.Sprintf("%d%%", int(frac*100))
	pw := font.MeasureString(smallFont, pct).Round()
	text.Draw(screen, pct, smallFont, int(x+w)-pw-10, int(y)+20, color.White)
}

func (g *Game) drawBossWarning(screen *ebiten.Image) {
	if g.bossWarn <= 0 {
		return
	}
	if (g.bossWarn/12)%2 == 0 {
		return
	}

	ebitenutil.DrawRect(screen, 0, 0, screenWidth, screenHeight, color.RGBA{255, 0, 0, 40})

	msg := "ALERTA DE CHEFÃO"
	sub := "PREPARE-SE"
	if g.bossStage == 1 {
		msg = "CHEFÃO FINAL"
		sub = "A BATALHA DECISIVA"
	}
	mw := font.MeasureString(titleFont, msg).Round()
	text.Draw(screen, msg, titleFont, screenWidth/2-mw/2, 440, color.RGBA{255, 70, 70, 255})

	sw := font.MeasureString(mediumFont, sub).Round()
	text.Draw(screen, sub, mediumFont, screenWidth/2-sw/2, 500, color.RGBA{255, 180, 120, 255})
}

func drawButton(screen *ebiten.Image, r Rect, label string, primary bool) {
	mx, my := ebiten.CursorPosition()
	hover := r.Contains(mx, my)

	bg := color.RGBA{18, 26, 46, 235}
	border := color.RGBA{70, 100, 160, 220}
	if primary {
		bg = color.RGBA{30, 90, 190, 245}
		border = color.RGBA{110, 170, 255, 255}
	}
	if hover {
		if primary {
			bg = color.RGBA{55, 130, 235, 255}
			border = color.RGBA{255, 255, 255, 255}
		} else {
			bg = color.RGBA{35, 50, 82, 245}
			border = color.RGBA{150, 190, 250, 255}
		}
	}

	ebitenutil.DrawRect(screen, r.X-2, r.Y-2, r.W+4, r.H+4, border)
	ebitenutil.DrawRect(screen, r.X, r.Y, r.W, r.H, bg)

	w := font.MeasureString(mediumFont, label).Round()
	text.Draw(screen, label, mediumFont, int(r.X+r.W/2)-w/2, int(r.Y+r.H/2)+11, color.White)
}

func drawLegend(screen *ebiten.Image, x, y float64, c color.RGBA, label string) {
	ebitenutil.DrawRect(screen, x, y, 22, 22, c)
	text.Draw(screen, label, smallFont, int(x)+32, int(y)+20, color.RGBA{170, 195, 235, 255})
}

func (g *Game) drawMenu(screen *ebiten.Image) {
	cx := screenWidth / 2

	title := "ASTEROIDE"
	tw := font.MeasureString(titleFont, title).Round()
	text.Draw(screen, title, titleFont, cx-tw/2+4, 154, color.RGBA{20, 60, 120, 255})
	text.Draw(screen, title, titleFont, cx-tw/2, 150, color.RGBA{90, 180, 255, 255})

	sub := "UM SHOOT 'EM UP ESPACIAL"
	sw := font.MeasureString(mediumFont, sub).Round()
	text.Draw(screen, sub, mediumFont, cx-sw/2, 195, color.RGBA{120, 150, 200, 255})

	section := "ESCOLHA SUA NAVE"
	secW := font.MeasureString(mediumFont, section).Round()
	text.Draw(screen, section, mediumFont, cx-secW/2, 235, color.RGBA{170, 195, 235, 255})

	for i := range ships {
		g.drawShipCard(screen, shipCardRects[i], i, i == g.selectedShip)
	}

	hint := "SETAS / 1-6 para escolher a nave"
	hw := font.MeasureString(smallFont, hint).Round()
	text.Draw(screen, hint, smallFont, cx-hw/2, 565, color.RGBA{140, 165, 205, 255})

	drawButton(screen, menuPlayRect, "JOGAR", true)
	drawButton(screen, menuQuitRect, "SAIR", false)

	rec := fmt.Sprintf("RECORDE  %d", g.bestScore)
	rw := font.MeasureString(mediumFont, rec).Round()
	text.Draw(screen, rec, mediumFont, cx-rw/2, 755, color.RGBA{255, 220, 100, 255})

	card := Rect{510, 780, 900, 270}
	drawPanel(screen, card, color.RGBA{8, 12, 26, 220}, color.RGBA{60, 95, 160, 200})

	title2 := "COMO JOGAR"
	t2w := font.MeasureString(mediumFont, title2).Round()
	text.Draw(screen, title2, mediumFont, cx-t2w/2, 816, color.White)

	text.Draw(screen, "WASD / SETAS   Mover a nave", smallFont, 560, 848, color.RGBA{170, 195, 235, 255})
	text.Draw(screen, "ESPAÇO         Atirar", smallFont, 560, 880, color.RGBA{170, 195, 235, 255})
	text.Draw(screen, "P / ESC        Pausar", smallFont, 560, 912, color.RGBA{170, 195, 235, 255})
	text.Draw(screen, "1 / 2 / 3 ...  Trocar de nave", smallFont, 560, 944, color.RGBA{170, 195, 235, 255})

	drawLegend(screen, 560, 975, color.RGBA{80, 255, 130, 255}, "Arma")
	drawLegend(screen, 760, 975, color.RGBA{255, 220, 70, 255}, "Cadência")
	drawLegend(screen, 960, 975, color.RGBA{90, 220, 255, 255}, "Escudo")
	drawLegend(screen, 1160, 975, color.RGBA{255, 90, 120, 255}, "Cura")
}

func (g *Game) drawShipCard(screen *ebiten.Image, r Rect, index int, selected bool) {
	mx, my := ebiten.CursorPosition()
	hover := r.Contains(mx, my)

	fill := color.RGBA{10, 16, 30, 225}
	border := color.RGBA{50, 70, 110, 220}
	if hover {
		fill = color.RGBA{18, 28, 50, 240}
	}
	if selected {
		fill = color.RGBA{22, 38, 68, 245}
		border = ships[index].tint
	}

	ebitenutil.DrawRect(screen, r.X-3, r.Y-3, r.W+6, r.H+6, border)
	ebitenutil.DrawRect(screen, r.X, r.Y, r.W, r.H, fill)

	img := g.shipImages[index]
	const scale = 0.48
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	w := float64(img.Bounds().Dx()) * scale
	op.GeoM.Translate(r.X+(r.W-w)/2, r.Y+8)
	screen.DrawImage(img, op)

	nw := font.MeasureString(smallFont, ships[index].name).Round()
	text.Draw(screen, ships[index].name, smallFont, int(r.X+r.W/2)-nw/2, int(r.Y)+90, color.White)

	tw := font.MeasureString(smallFont, ships[index].trait).Round()
	text.Draw(screen, ships[index].trait, smallFont, int(r.X+r.W/2)-tw/2, int(r.Y)+114, ships[index].tint)
}

func (g *Game) drawPause(screen *ebiten.Image) {
	ebitenutil.DrawRect(screen, 0, 0, screenWidth, screenHeight, color.RGBA{0, 0, 0, 175})
	cx := screenWidth / 2

	w := font.MeasureString(titleFont, "PAUSADO").Round()
	text.Draw(screen, "PAUSADO", titleFont, cx-w/2, 400, color.RGBA{140, 200, 255, 255})

	drawButton(screen, pauseResumeRect, "CONTINUAR", true)
	drawButton(screen, pauseRestartRect, "REINICIAR", false)
	drawButton(screen, pauseMenuRect, "MENU", false)

	hint := "P / ESC continuar   -   R reiniciar   -   M menu"
	hw := font.MeasureString(smallFont, hint).Round()
	text.Draw(screen, hint, smallFont, cx-hw/2, 740, color.RGBA{150, 175, 215, 255})
}

func (g *Game) drawGameOver(screen *ebiten.Image) {
	ebitenutil.DrawRect(screen, 0, 0, screenWidth, screenHeight, color.RGBA{0, 0, 0, 185})
	cx := screenWidth / 2

	title := "FIM DE JOGO!"
	c := color.RGBA{255, 90, 90, 255}
	if g.won {
		title = "VOCÊ VENCEU!"
		c = color.RGBA{90, 255, 150, 255}
	}
	w := font.MeasureString(titleFont, title).Round()
	text.Draw(screen, title, titleFont, cx-w/2, 320, c)

	lines := []string{
		fmt.Sprintf("PONTUAÇÃO  %d", g.score),
		fmt.Sprintf("INIMIGOS   %d", g.destroyed),
		fmt.Sprintf("RECORDE    %d", g.bestScore),
	}
	for i, line := range lines {
		lw := font.MeasureString(mediumFont, line).Round()
		text.Draw(screen, line, mediumFont, cx-lw/2, 430+i*54, color.White)
	}

	drawButton(screen, overAgainRect, "JOGAR NOVAMENTE", true)
	drawButton(screen, overMenuRect, "MENU", false)

	hint := "ENTER jogar novamente   -   M menu"
	hw := font.MeasureString(smallFont, hint).Round()
	text.Draw(screen, hint, smallFont, cx-hw/2, 800, color.RGBA{150, 175, 215, 255})
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

var playerAlpha [][]bool

func scaleShipImage(src image.Image) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, playerWidth, playerHeight))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)
	return dst
}

func loadPlayerAlpha(img image.Image) {
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	playerAlpha = make([][]bool, height)
	for y := 0; y < height; y++ {
		playerAlpha[y] = make([]bool, width)
		for x := 0; x < width; x++ {
			_, _, _, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			playerAlpha[y][x] = a > 128
		}
	}
}

func loadShipImages(path string) []*ebiten.Image {
	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	img, err := png.Decode(file)
	file.Close()
	if err != nil {
		log.Fatal(err)
	}
	img = scaleShipImage(img)
	loadPlayerAlpha(img)

	images := make([]*ebiten.Image, len(ships))
	for i, s := range ships {
		images[i] = ebiten.NewImageFromImage(recolorShip(img, s.tint))
	}
	return images
}

func recolorShip(src image.Image, tint color.RGBA) image.Image {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	tr, tg, tb := float64(tint.R), float64(tint.G), float64(tint.B)

	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			r, g, bl, a := src.At(b.Min.X+x, b.Min.Y+y).RGBA()
			if a == 0 {
				continue
			}
			lum := (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)) / 65535.0
			shade := (0.42 + 0.58*lum) * float64(a) / 65535.0
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(tr * shade),
				G: uint8(tg * shade),
				B: uint8(tb * shade),
				A: uint8(a >> 8),
			})
		}
	}
	return dst
}

func playerHitsEnemy(px, py float64, e Enemy) bool {
	ex, ey := int(e.x), int(e.y)
	es := int(e.size)
	ipx, ipy := int(px), int(py)

	if ipx+playerWidth < ex || ipx > ex+es || ipy+playerHeight < ey || ipy > ey+es {
		return false
	}

	for y := 0; y < len(playerAlpha); y++ {
		screenY := ipy + y
		if screenY < ey || screenY >= ey+es {
			continue
		}
		row := playerAlpha[y]
		for x := 0; x < len(row); x++ {
			if !row[x] {
				continue
			}
			screenX := ipx + x
			if screenX >= ex && screenX < ex+es {
				return true
			}
		}
	}
	return false
}

func bulletHitsEnemy(b Bullet, e Enemy) bool {
	return b.x-b.w/2 < e.x+e.size &&
		b.x+b.w/2 > e.x &&
		b.y-b.h/2 < e.y+e.size &&
		b.y+b.h/2 > e.y
}

func bulletHitsBoss(b Bullet, boss *Boss) bool {
	return b.x-b.w/2 < boss.x+boss.w &&
		b.x+b.w/2 > boss.x &&
		b.y-b.h/2 < boss.y+boss.h &&
		b.y+b.h/2 > boss.y
}

func circleHitsPlayer(eb EnemyBullet, px, py float64) bool {
	hx := px + (playerWidth-playerHitW)/2
	hy := py + (playerHeight-playerHitH)/2
	return rectsOverlap(eb.x-eb.r, eb.y-eb.r, eb.r*2, eb.r*2, hx, hy, playerHitW, playerHitH)
}

func rectsOverlap(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	game := newGame()
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("ASTEROIDE - Shoot 'em Up")
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
