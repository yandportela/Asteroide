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
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
)

const (
	screenWidth  = 1920
	screenHeight = 1080

	playerWidth  = 128
	playerHeight = 128
	playerSpeed  = 8.0

	bulletWidth  = 10
	bulletHeight = 24

	maxLives    = 3
	maxPower    = 5
	maxRapid    = 3
	maxGameTime = 120.0

	enemyMinSize = 40.0
	enemyMaxSize = 78.0

	invulnFrames = 110
	shieldFrames = 600

	bossTriggerTime = 60.0
	bossWarnFrames  = 120
	bossMaxHP       = 500
	bossTargetY     = 130.0
)

const (
	powerWeapon = iota
	powerRapid
	powerShield
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

var (
	menuPlayRect     = Rect{800, 360, 320, 72}
	menuQuitRect     = Rect{800, 452, 320, 72}
	pauseResumeRect  = Rect{800, 470, 320, 64}
	pauseRestartRect = Rect{800, 550, 320, 64}
	pauseMenuRect    = Rect{800, 630, 320, 64}
	overAgainRect    = Rect{770, 610, 380, 64}
	overMenuRect     = Rect{800, 690, 320, 64}
)

type Bullet struct {
	x, y   float64
	speedY float64
	dx     float64
	damage int
}

type Enemy struct {
	x, y   float64
	size   float64
	speedY float64
	driftX float64
	hp     int
	kind   int
}

type Boss struct {
	x, y      float64
	w, h      float64
	hp, maxHP int
	vx        float64
	fireTimer int
	volley    int
	hitFlash  int
	entering  bool
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
	player  *ebiten.Image

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
	powerLevel      int
	rapidLevel      int
	shieldTimer     int
	invulnTimer     int
	fireCooldown    int
	combo           int
	comboTimer      int
	bossScheduled   bool
	bossWarn        int
	bossFightFrames int

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
	g := &Game{player: loadImage("player.png")}
	g.resetState()
	g.state = stateMenu
	return g
}

func (g *Game) resetState() {
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
	g.frameCount = 0
	g.score = 0
	g.destroyed = 0
	g.lives = maxLives
	g.powerLevel = 0
	g.rapidLevel = 0
	g.shieldTimer = 0
	g.invulnTimer = 0
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
			g.finishGame(true)
			return nil
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
		g.playerX += mx / length * playerSpeed
		g.playerY += my / length * playerSpeed
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

func (g *Game) handleShooting() {
	if !ebiten.IsKeyPressed(ebiten.KeySpace) || g.fireCooldown > 0 {
		return
	}

	cx := g.playerX + playerWidth/2 - bulletWidth/2
	cy := g.playerY + 18
	speed := 13.0

	switch g.powerLevel {
	case 0:
		g.addBullet(cx, cy, 0, speed)
	case 1:
		g.addBullet(cx-12, cy, 0, speed)
		g.addBullet(cx+12, cy, 0, speed)
	case 2:
		g.addBullet(cx, cy, 0, speed)
		g.addBullet(cx-20, cy+6, -1.7, speed)
		g.addBullet(cx+20, cy+6, 1.7, speed)
	case 3:
		g.addBullet(cx-8, cy, 0, speed)
		g.addBullet(cx+8, cy, 0, speed)
		g.addBullet(cx-28, cy+8, -2.6, speed*0.97)
		g.addBullet(cx+28, cy+8, 2.6, speed*0.97)
	default:
		g.addBullet(cx, cy-6, 0, speed+1)
		g.addBullet(cx-16, cy, -1.2, speed)
		g.addBullet(cx+16, cy, 1.2, speed)
		g.addBullet(cx-32, cy+8, -3.0, speed*0.95)
		g.addBullet(cx+32, cy+8, 3.0, speed*0.95)
	}

	cooldown := 12 - g.rapidLevel*2
	if cooldown < 6 {
		cooldown = 6
	}
	g.fireCooldown = cooldown
}

func (g *Game) addBullet(x, y, dx, speedY float64) {
	g.bullets = append(g.bullets, Bullet{x: x, y: y, dx: dx, speedY: speedY, damage: 1})
}

func (g *Game) updateBullets() {
	alive := g.bullets[:0]
	for _, b := range g.bullets {
		b.y -= b.speedY
		b.x += b.dx
		if b.y > -bulletHeight-10 && b.x > -40 && b.x < screenWidth+40 {
			alive = append(alive, b)
		}
	}
	g.bullets = alive
}

func (g *Game) spawnEnemies() {
	g.frameCount++
	if g.bossWarn > 0 || g.boss != nil {
		return
	}
	interval := int(52 - g.gameTime*0.28)
	if interval < 12 {
		interval = 12
	}
	if g.frameCount%interval != 0 {
		return
	}

	var size float64
	var hp, kind int
	r := rand.Float64()
	switch {
	case r < 0.62:
		size = enemyMinSize
		hp = 1
		kind = 0
	case r < 0.9:
		size = 58
		hp = 2
		kind = 1
	default:
		size = enemyMaxSize
		hp = 4
		kind = 2
	}

	speed := 1.4 + rand.Float64()*1.6 + g.gameTime*0.012
	drift := (rand.Float64()*2 - 1) * (0.6 + g.gameTime*0.004)

	g.enemies = append(g.enemies, Enemy{
		x:      rand.Float64() * (screenWidth - size),
		y:      -size,
		size:   size,
		speedY: speed - float64(kind)*0.15,
		driftX: drift,
		hp:     hp,
		kind:   kind,
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
		if e.y < screenHeight+e.size {
			alive = append(alive, e)
		}
	}
	g.enemies = alive
}

func (g *Game) resolveBulletHits() {
	survivingBullets := g.bullets[:0]
	for _, b := range g.bullets {
		hit := false
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
				g.addExplosion(b.x+bulletWidth/2, b.y, 14, 8, 255, 240, 160)
			}
			break
		}
		if !hit && g.boss != nil && !g.boss.entering && bulletHitsBoss(b, g.boss) {
			g.boss.hp -= b.damage
			g.boss.hitFlash = 3
			hit = true
			g.addExplosion(b.x+bulletWidth/2, b.y, 12, 6, 255, 200, 120)
			if g.boss.hp <= 0 {
				g.defeatBoss()
			}
		}
		if !hit {
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
	w, h := 300.0, 170.0
	g.boss = &Boss{
		x:         screenWidth/2 - w/2,
		y:         -h,
		w:         w,
		h:         h,
		hp:        bossMaxHP,
		maxHP:     bossMaxHP,
		vx:        2.5,
		fireTimer: 80,
		entering:  true,
	}
	g.enemyBullets = nil
	g.shake = 12
}

func (g *Game) updateBoss() {
	if !g.bossScheduled {
		if g.gameTime < bossTriggerTime {
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
	speed := 2.0 + (1.0-hpFrac)*3.0
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
	b.y = bossTargetY + math.Sin(float64(g.tick)*0.03)*12

	b.fireTimer--
	if b.fireTimer <= 0 {
		g.bossShoot()
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

	speed := 5.0
	count := 3
	spread := 0.16
	if hpFrac < 0.7 {
		speed = 5.5
		count = 5
	}
	if hpFrac < 0.35 {
		speed = 6.0
		count = 7
	}

	b.volley++
	if b.volley%3 == 0 {
		n := 14
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

	b.fireTimer = 80
	if hpFrac < 0.7 {
		b.fireTimer = 60
	}
	if hpFrac < 0.35 {
		b.fireTimer = 42
	}
	b.fireTimer += rand.Intn(15)
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

	g.score += 1000
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

	g.boss = nil
	g.enemyBullets = nil
	g.startTime = g.startTime.Add(time.Duration(g.bossFightFrames) * time.Second / time.Duration(ebiten.TPS()))
	g.bossFightFrames = 0
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
	}
	points += int(g.gameTime / 20)

	g.combo++
	g.comboTimer = 150
	g.score += points * comboMultiplier(g.combo)

	g.destroyed++
	g.addExplosion(e.x+e.size/2, e.y+e.size/2, e.size*0.9, 22, 255, 170, 40)
	g.shake = maxInt(g.shake, 3+int(e.size/20))

	if rand.Intn(100) < 14 {
		g.powerUps = append(g.powerUps, PowerUp{
			x:    e.x + e.size/2 - 12,
			y:    e.y + e.size/2 - 12,
			kind: rand.Intn(3),
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
		if rectsOverlap(p.x, p.y, 24, 24, g.playerX, g.playerY, playerWidth, playerHeight) {
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
	screen.DrawImage(g.player, op)
}

func (g *Game) drawBoss(screen *ebiten.Image, sx, sy float64) {
	b := g.boss
	if b == nil {
		return
	}
	x, y := b.x+sx, b.y+sy

	wing := color.RGBA{70, 22, 96, 255}
	body := color.RGBA{110, 42, 140, 255}
	if b.hitFlash > 0 {
		wing = color.RGBA{210, 200, 230, 255}
		body = color.RGBA{255, 255, 255, 255}
	}

	ebitenutil.DrawRect(screen, x, y+45, b.w, 70, wing)
	ebitenutil.DrawRect(screen, x+50, y+30, b.w-100, b.h-50, wing)
	ebitenutil.DrawRect(screen, x+70, y, b.w-140, b.h, body)
	ebitenutil.DrawRect(screen, x+92, y+16, b.w-184, b.h-32, color.RGBA{140, 60, 170, 255})

	ebitenutil.DrawRect(screen, x+55, y+b.h-42, 42, 48, color.RGBA{45, 45, 65, 255})
	ebitenutil.DrawRect(screen, x+b.w-97, y+b.h-42, 42, 48, color.RGBA{45, 45, 65, 255})

	pulse := 0.5 + 0.5*math.Sin(float64(g.tick)*0.15)
	core := uint8(140 + 110*pulse)
	ebitenutil.DrawCircle(screen, x+b.w/2, y+b.h/2, 36, color.RGBA{core, 50, 210, 255})
	ebitenutil.DrawCircle(screen, x+b.w/2, y+b.h/2, 16, color.RGBA{255, 245, 255, 255})

	ebitenutil.DrawCircle(screen, x+88, y+62, 10, color.RGBA{255, 70, 70, 255})
	ebitenutil.DrawCircle(screen, x+b.w-88, y+62, 10, color.RGBA{255, 70, 70, 255})

	ebitenutil.DrawRect(screen, x+b.w/2-3, y-14, 6, 18, color.RGBA{200, 80, 240, 255})
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
		default:
			c = color.RGBA{240, 70, 70, 255}
		}
		ebitenutil.DrawRect(screen, e.x+sx, e.y+sy, e.size, e.size, c)
		inner := e.size * 0.55
		ic := color.RGBA{c.R / 2, c.G / 2, c.B / 2, 255}
		ebitenutil.DrawRect(screen, e.x+sx+(e.size-inner)/2, e.y+sy+(e.size-inner)/2, inner, inner, ic)
	}
}

func (g *Game) drawBullets(screen *ebiten.Image, sx, sy float64) {
	for _, b := range g.bullets {
		ebitenutil.DrawRect(screen, b.x+sx-2, b.y+sy, bulletWidth+4, bulletHeight, color.RGBA{40, 120, 255, 80})
		ebitenutil.DrawRect(screen, b.x+sx, b.y+sy, bulletWidth, bulletHeight, color.RGBA{120, 230, 255, 255})
		ebitenutil.DrawRect(screen, b.x+sx+2, b.y+sy+4, bulletWidth-4, bulletHeight-8, color.White)
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
		default:
			c = color.RGBA{90, 220, 255, 255}
		}
		size := 22 + 4*math.Sin(float64(g.tick)*0.18)
		offset := (28 - size) / 2
		ebitenutil.DrawRect(screen, p.x+sx+offset-3, p.y+sy+offset-3, size+6, size+6, color.RGBA{c.R, c.G, c.B, 90})
		ebitenutil.DrawRect(screen, p.x+sx+offset, p.y+sy+offset, size, size, c)
		ebitenutil.DrawRect(screen, p.x+sx+10, p.y+sy+10, 8, 8, color.White)
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
	text.Draw(screen, fmt.Sprintf("%02d:%02d", tempo/60, tempo%60), smallFont, 200, 58, color.White)
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

	text.Draw(screen, "CHEFÃO", smallFont, int(x)+10, int(y)+20, color.White)
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
	mw := font.MeasureString(titleFont, msg).Round()
	text.Draw(screen, msg, titleFont, screenWidth/2-mw/2, 440, color.RGBA{255, 70, 70, 255})

	sub := "PREPARE-SE"
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
	text.Draw(screen, title, titleFont, cx-tw/2+4, 234, color.RGBA{20, 60, 120, 255})
	text.Draw(screen, title, titleFont, cx-tw/2, 230, color.RGBA{90, 180, 255, 255})

	sub := "UM SHOOT 'EM UP ESPACIAL"
	sw := font.MeasureString(mediumFont, sub).Round()
	text.Draw(screen, sub, mediumFont, cx-sw/2, 290, color.RGBA{120, 150, 200, 255})

	drawButton(screen, menuPlayRect, "JOGAR", true)
	drawButton(screen, menuQuitRect, "SAIR", false)

	hint := "ENTER / ESPAÇO para jogar   -   clique nos botões"
	hw := font.MeasureString(smallFont, hint).Round()
	text.Draw(screen, hint, smallFont, cx-hw/2, 570, color.RGBA{140, 165, 205, 255})

	rec := fmt.Sprintf("RECORDE  %d", g.bestScore)
	rw := font.MeasureString(mediumFont, rec).Round()
	text.Draw(screen, rec, mediumFont, cx-rw/2, 640, color.RGBA{255, 220, 100, 255})

	card := Rect{580, 680, 760, 300}
	drawPanel(screen, card, color.RGBA{8, 12, 26, 220}, color.RGBA{60, 95, 160, 200})

	title2 := "COMO JOGAR"
	t2w := font.MeasureString(mediumFont, title2).Round()
	text.Draw(screen, title2, mediumFont, cx-t2w/2, 724, color.White)

	text.Draw(screen, "WASD / SETAS   Mover a nave", smallFont, 650, 772, color.RGBA{170, 195, 235, 255})
	text.Draw(screen, "ESPAÇO         Atirar", smallFont, 650, 810, color.RGBA{170, 195, 235, 255})
	text.Draw(screen, "P / ESC        Pausar", smallFont, 650, 848, color.RGBA{170, 195, 235, 255})

	drawLegend(screen, 650, 892, color.RGBA{80, 255, 130, 255}, "Arma")
	drawLegend(screen, 900, 892, color.RGBA{255, 220, 70, 255}, "Cadência")
	drawLegend(screen, 1150, 892, color.RGBA{90, 220, 255, 255}, "Escudo")
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
	return b.x < e.x+e.size &&
		b.x+bulletWidth > e.x &&
		b.y < e.y+e.size &&
		b.y+bulletHeight > e.y
}

func bulletHitsBoss(b Bullet, boss *Boss) bool {
	return b.x < boss.x+boss.w &&
		b.x+bulletWidth > boss.x &&
		b.y < boss.y+boss.h &&
		b.y+bulletHeight > boss.y
}

func circleHitsPlayer(eb EnemyBullet, px, py float64) bool {
	return rectsOverlap(eb.x-eb.r, eb.y-eb.r, eb.r*2, eb.r*2, px, py, playerWidth, playerHeight)
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
