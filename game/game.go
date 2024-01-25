package game

import (
	"astrogame/assets"
	"astrogame/config"
	"astrogame/objects"
	"fmt"
	"image/color"
	"math/rand"
	"slices"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font"
)

type options struct {
	fullscreen          bool
	ResolutionMultipler float64
	ScreenWidth         float64
	ScreenHeight        float64
	ScreenXMenuShift    int
	ScreenYMenuShift    int
	ScreenXProfileShift int
	ScreenYProfileShift int
	ScreenYMenuHeight   int
	ScreenFontHeight    int
	ScreenFontWidth     int
	ScoreFont           font.Face
	InfoFont            font.Face
	SmallFont           font.Face
	ProfileFont         font.Face
	ProfileBigFont      font.Face
}
type Game struct {
	Options           *options
	menu              *MainMenu
	optionsMenu       *OptionsMenu
	profile           *ProfileScreen
	state             config.GameState
	player            *Player
	meteorSpawnTimer  *config.Timer
	meteors           []*Meteor
	projectiles       []*Projectile
	blows             []*Blow
	beams             []*Beam
	beamAnimations    []*BeamAnimation
	enemyBeams        []*Beam
	enemyProjectiles  []*Projectile
	enemies           []*Enemy
	items             []*Item
	animations        []*Animation
	bgImage           *ebiten.Image
	score             int
	viewport          viewport
	levels            []*config.Level
	curLevel          *config.Level
	CurStage          *config.Stage
	baseVelocity      float64
	velocityTimer     *config.Timer
	enemySpawnTimer   *config.Timer
	batchesSpawnTimer *config.Timer
	itemSpawnTimer    *config.Timer
	CurWave           *config.Wave
	started           bool
}

func NewGame() *Game {
	g := &Game{
		Options: &options{
			fullscreen:          false,
			ResolutionMultipler: 1,
			ScreenWidth:         config.ScreenWidth1024X768,
			ScreenHeight:        config.ScreenHeight1024X768,
			ScreenXMenuShift:    config.Screen1024X768XMenuShift,
			ScreenYMenuShift:    config.Screen1024X768YMenuShift,
			ScreenXProfileShift: config.Screen1024X768XProfileShift,
			ScreenYProfileShift: config.Screen1024X768YProfileShift,
			ScreenYMenuHeight:   config.Screen1024X768YMenuHeight,
			ScreenFontHeight:    config.Screen1024X768FontHeight,
			ScreenFontWidth:     config.Screen1024X768FontWidth,
			ScoreFont:           assets.ScoreFont1024x768,
			InfoFont:            assets.InfoFont1024x768,
			SmallFont:           assets.SmallFont1024x768,
			ProfileFont:         assets.ProfileFont1024x768,
			ProfileBigFont:      assets.ProfileBigFont1024x768,
		},
		state:             config.MainMenu,
		meteorSpawnTimer:  config.NewTimer(config.MeteorSpawnTime),
		baseVelocity:      config.BaseMeteorVelocity,
		velocityTimer:     config.NewTimer(config.MeteorSpeedUpTime),
		enemySpawnTimer:   config.NewTimer(config.Levels[0].Stages[0].Waves[0].Batches[0].Type.EnemySpawnTime),
		batchesSpawnTimer: config.NewTimer(config.Levels[0].Stages[0].Waves[0].Batches[0].BatchSpawnTime),
		itemSpawnTimer:    config.NewTimer(config.Levels[0].Stages[0].Items[0].ItemSpawnTime),
		bgImage:           config.Levels[0].BgImg,
		levels:            config.Levels,
		curLevel:          config.Levels[0],
		CurStage:          &config.Levels[0].Stages[0],
		CurWave:           &config.Levels[0].Stages[0].Waves[0],
		started:           false,
	}

	g.player = NewPlayer(g)
	g.menu = NewMainMenu(g)
	g.optionsMenu = NewOptionsMenu(g)
	g.profile = NewPlayerProfile(g)

	return g
}

type viewport struct {
	X int
	Y int
}

func (g *Game) BgMove() {
	s := g.bgImage.Bounds().Size()
	maxX := s.X * 50
	maxY := s.Y * 50

	g.viewport.X += s.X / 100
	g.viewport.Y += s.Y / 100
	g.viewport.X %= maxX
	g.viewport.Y %= maxY
}

func (g *Game) BgPosition() (int, int) {
	return g.viewport.X, g.viewport.Y
}
func (g *Game) MoveBgPosition() {
	g.BgMove()
	g.velocityTimer.Update()
	if g.velocityTimer.IsReady() {
		g.velocityTimer.Reset()
		g.baseVelocity += config.MeteorSpeedUpAmount
	}
}

func (g *Game) Update() error {
	g.MoveBgPosition()
	switch g.state {
	case config.Options:
		err := g.optionsMenu.Update()
		if err != nil {
			return err
		}
	case config.Profile:
		g.profile.Update()
	case config.MainMenu:
		err := g.menu.Update()
		if err != nil {
			return err
		}
	case config.InGame:
		// Main menu logic
		if !g.started {
			g.started = true
			// Unlock the continue button
			for i := range g.menu.Items {
				if g.menu.Items[i].Label == "Continue game" {
					g.menu.Items[i].Active = true
				}
			}
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			g.state = config.MainMenu
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyP) {
			for _, i := range g.profile.LeftBar.Items {
				i.UpdatePrevValue(g)
			}
			for _, i := range g.profile.RightBar.Items {
				i.UpdatePrevValue(g)
			}
			g.state = config.Profile
		}

		// Game logic
		g.player.Update()

		// Meteor spawning
		g.meteorSpawnTimer.Update()
		if g.meteorSpawnTimer.IsReady() {
			g.meteorSpawnTimer.Reset()
			if g.CurStage.MeteorsCount > 0 {
				m := NewMeteor(g.baseVelocity, g)
				g.meteors = append(g.meteors, m)
				g.CurStage.MeteorsCount--
			}
		}

		// Item spawning
		g.itemSpawnTimer.Update()
		if g.itemSpawnTimer.IsReady() {
			if len(g.CurStage.Items) > 0 {
				var target config.Vector
				var startPos config.Vector
				itemParam := g.CurStage.Items[0]
				item := NewItem(g, target, startPos, &itemParam)
				if len(g.CurStage.Items) > 1 {
					g.itemSpawnTimer.Restart(g.CurStage.Items[1].ItemSpawnTime)
				}
				//g.itemSpawnTimer.Restart(g.CurStage.Items[1].ItemSpawnTime)
				itemWidth := item.itemType.Sprite.Bounds().Dx()
				itemHight := item.itemType.Sprite.Bounds().Dy()
				r := rand.New(rand.NewSource(99))
				posX := r.Intn(int(g.Options.ScreenWidth) - (itemWidth + itemWidth/2))
				startPos = config.Vector{
					X: float64(posX + itemWidth),
					Y: -(float64(itemHight)),
				}
				target = config.Vector{
					X: startPos.X,
					Y: g.Options.ScreenHeight + 10,
				}
				item.SetDirection(target, startPos, &itemParam)
				item.target = target
				g.items = append(g.items, item)
				g.CurStage.Items = slices.Delete(g.CurStage.Items, 0, 1)
			}
		}

		// Enemy spawning
		g.batchesSpawnTimer.Update()
		if g.batchesSpawnTimer.IsReady() {
			if len(g.CurWave.Batches) > 0 {
				batch := g.CurWave.Batches[0]
				if len(g.CurWave.Batches) > 1 {
					g.batchesSpawnTimer = config.NewTimer(g.CurWave.Batches[1].BatchSpawnTime)
				}
				g.batchesSpawnTimer.Reset()
				elemInLineCount := 0
				linesCount := 0.0
				var xOffsetMod float64
				if batch.StartPositionType == "centered" {
					eTst := NewEnemy(g, config.Vector{}, config.Vector{}, *batch.Type)
					enemyWidthTst := eTst.enemyType.Sprite.Bounds().Dx()
					xOffsetMod = (g.Options.ScreenWidth - float64(batch.Count*(enemyWidthTst+int(batch.StartPosOffset))-int(batch.StartPosOffset))) / 2
				}
				for i := 0; i < batch.Count; i++ {
					var target config.Vector
					var startPos config.Vector
					e := NewEnemy(g, target, startPos, *batch.Type)
					e.enemyType.Sprite = objects.ScaleImg(e.enemyType.Sprite, g.Options.ResolutionMultipler)
					e.TargetType = batch.TargetType
					enemyWidth := e.enemyType.Sprite.Bounds().Dx()
					enemyHight := e.enemyType.Sprite.Bounds().Dy()
					switch batch.StartPositionType {
					case "centered":
						xOffset := batch.StartPosOffset
						elemInLine := int(g.Options.ScreenWidth) / (enemyWidth + int(xOffset))
						if elemInLineCount == 0 {
							xOffset = 0.0
						}
						if elemInLineCount >= elemInLine {
							elemInLineCount = 0
							linesCount++
						}
						startPos = config.Vector{
							X: (float64(enemyWidth)+xOffset)*float64(elemInLineCount) + xOffsetMod,
							Y: -(float64(enemyHight)*linesCount + 10),
						}
					case "lines":
						xOffset := batch.StartPosOffset
						elemInLine := int(g.Options.ScreenWidth) / (enemyWidth + int(xOffset))
						if elemInLineCount == 0 {
							xOffset = 0.0
						}
						if elemInLineCount >= elemInLine {
							elemInLineCount = 0
							linesCount++
						}
						startPos = config.Vector{
							X: (float64(enemyWidth) + xOffset) * float64(elemInLineCount),
							Y: -(float64(enemyHight)*linesCount + 10),
						}
					case "checkmate":
						elemInLine := int(g.Options.ScreenWidth) / (enemyWidth * 2)
						if elemInLineCount >= elemInLine {
							elemInLineCount = 0
							linesCount++
						}
						startPos = config.Vector{
							X: float64(enemyWidth)*float64(elemInLineCount)*2 + float64(enemyWidth),
							Y: -(float64(enemyHight)*linesCount + 10),
						}
						if int(linesCount)%2 == 0 {
							startPos = config.Vector{
								X: float64(enemyWidth) * float64(elemInLineCount) * 2,
								Y: -(float64(enemyHight)*linesCount + 10),
							}
						}
					}
					switch batch.TargetType {
					case "player":
						target = config.Vector{
							X: g.player.position.X,
							Y: g.player.position.Y,
						}
					case "straight":
						target = config.Vector{
							X: startPos.X,
							Y: g.Options.ScreenHeight + 10,
						}
					}
					elemInLineCount++
					e.SetDirection(target, startPos, *batch.Type)
					e.target = target
					g.enemies = append(g.enemies, e)
				}
				g.CurWave.Batches = slices.Delete(g.CurWave.Batches, 0, 1)
			}
		}

		if len(g.CurWave.Batches) == 0 {
			if g.CurWave.WaveId < len(g.CurStage.Waves)-1 {
				g.CurWave = &g.CurStage.Waves[g.CurWave.WaveId+1]
			} else {
				if g.CurStage.MeteorsCount == 0 && g.CurStage.StageId < len(g.curLevel.Stages)-1 && len(g.CurStage.Items) == 0 {
					g.CurStage = &g.curLevel.Stages[g.CurStage.StageId+1]
					g.CurWave = &g.CurStage.Waves[0]
				} else {
					if g.curLevel.LevelId < len(g.levels)-1 {
						g.curLevel = g.levels[g.curLevel.LevelId+1]
						g.CurStage = &g.curLevel.Stages[0]
						g.CurWave = &g.CurStage.Waves[0]
					} else {
						g.Reset()
					}
				}
			}
		}

		for i, m := range g.meteors {
			m.Update()
			if m.Collider().Min.Y >= int(g.Options.ScreenHeight) && i < len(g.meteors) {
				g.meteors = slices.Delete(g.meteors, i, i+1)
			}
		}

		for i, p := range g.projectiles {
			p.Update()
			if i < len(g.projectiles) && (p.position.Y < 0 || p.position.Y >= g.Options.ScreenHeight+float64(p.wType.Sprite.Bounds().Dy())) {
				g.projectiles[i].Destroy(g, i)
			} else if i < len(g.projectiles) && (p.position.X < 0 || p.position.X > g.Options.ScreenWidth+float64(p.wType.Sprite.Bounds().Dx())) {
				g.projectiles[i].Destroy(g, i)
			}
		}

		for i, p := range g.enemyProjectiles {
			if p.wType.TargetType == "auto" && p.owner == "enemy" {
				p.target = config.Vector{
					X: g.player.position.X,
					Y: g.player.position.Y,
				}
			}
			p.Update()
			if p.position.Y >= g.Options.ScreenHeight && i < len(g.enemyProjectiles) {
				g.enemyProjectiles = slices.Delete(g.enemyProjectiles, i, i+1)
			}
		}

		for i, item := range g.items {
			item.Update()
			if item.position.Y >= g.Options.ScreenHeight && i < len(g.items) {
				g.items = slices.Delete(g.items, i, i+1)
			}
		}

		// Check for meteor/projectile collisions
		// Check for meteor/enemy projectile collisions
		for i, m := range g.meteors {
			for j, b := range g.projectiles {
				if config.IntersectRect(m.Collider(), b.Collider()) {
					if (i < len(g.meteors)) && (j < len(g.projectiles)) {
						g.meteors = append(g.meteors[:i], g.meteors[i+1:]...)
						g.IntersectProjectile(b, j)
						g.score++
					}
				}
			}

			for j, b := range g.enemyProjectiles {
				if config.IntersectRect(m.Collider(), b.Collider()) {
					if (i < len(g.meteors)-1) && (j < len(g.projectiles)-1) {
						g.IntersectProjectile(b, j)
					}
				}
			}
		}

		// Check for enemy/projectile collisions
		// Check for enemy/player collisions
		// Check for enemy/beam collisions
		// Check for enemy/blow collisions
		for i, m := range g.enemies {
			if m.TargetType == config.TargetTypePlayer {
				m.target = config.Vector{
					X: g.player.position.X,
					Y: g.player.position.Y,
				}
			}
			m.Update()
			if m.position.Y >= g.Options.ScreenHeight+float64(m.Collider().Dy()) && i < len(g.enemies) {
				g.enemies = slices.Delete(g.enemies, i, i+1)
			}

			for j, b := range g.projectiles {
				if config.IntersectRect(m.Collider(), b.Collider()) && b.owner == "player" {
					switch b.wType.WeaponName {
					case config.BigBomb:
						bounds := b.wType.Sprite.Bounds()
						blow := NewBlow(b.position.X+float64(bounds.Dx()/2), b.position.Y+float64(bounds.Dy()/2), float64(bounds.Dx())*4, b.wType.Damage)
						blow.Steps = 5
						g.AddBlow(blow, m.position)
					default:
						m.HP -= b.wType.Damage
						if m.HP <= 0 {
							if (i < len(g.enemies)) && (j < len(g.projectiles)) {
								g.KillEnemy(i)
							}
						}
					}

					if j < len(g.projectiles) {
						g.projectiles[j].intercectAnimation.position = b.position
						g.projectiles[j].AddAnimation(g)
						g.projectiles[j].HP--
						if g.projectiles[j].HP <= 0 {
							g.projectiles[j].Destroy(g, j)
						}
					}
				}
			}

			for _, blow := range g.blows {
				if config.IntersectCircle(m.Collider(), blow.circle) {
					m.HP -= blow.Damage
					if m.HP <= 0 {
						if i < len(g.enemies) {
							g.KillEnemy(i)
						}
					}
				}
			}

			if config.IntersectRect(m.Collider(), g.player.Collider()) {
				if g.player.shield != nil {
					g.player.shield = nil
				} else {
					g.Reset()
					break
				}
			}

			for _, beam := range g.beams {
				if config.IntersectLine(beam.Line, m.Collider()) {
					m.HP -= beam.Damage
					if m.HP <= 0 {
						if i < len(g.enemies) {
							g.KillEnemy(i)
						}
					}
				}
			}
		}

		// Check for enemy projectile/player projectile collisions
		for i, m := range g.enemyProjectiles {
			for j, b := range g.projectiles {
				if config.IntersectRect(m.Collider(), b.Collider()) {
					if (i < len(g.enemyProjectiles)) && (j < len(g.projectiles)) {
						g.IntersectProjectile(m, i)
						g.IntersectProjectile(b, j)
					}
				}
			}
			for _, beam := range g.beams {
				if config.IntersectLine(beam.Line, m.Collider()) {
					if i < len(g.enemyProjectiles) {
						g.IntersectProjectile(m, i)
					}
				}
			}
		}

		// Check for projectiles/player collisions
		for i, p := range g.enemyProjectiles {
			if config.IntersectRect(p.Collider(), g.player.Collider()) {
				if g.player.shield != nil {
					g.player.shield.HP -= p.wType.Damage
					if g.player.shield.HP <= 0 {
						g.player.shield = nil
					}
				} else {
					g.player.params.HP -= p.wType.Damage
				}
				if i < len(g.enemyProjectiles) {
					g.IntersectProjectile(p, i)
				}
				if g.player.params.HP <= 0 {
					g.Reset()
					break
				}
			}
		}

		// Check for meteor/player collisions
		for _, m := range g.meteors {
			if config.IntersectRect(m.Collider(), g.player.Collider()) {
				g.Reset()
				break
			}
		}

		// Check for item/player collisions
		for i, item := range g.items {
			if config.IntersectRect(item.Collider(), g.player.Collider()) {
				item.CollideWithPlayer(g.player)
				if i < len(g.items) {
					g.items = append(g.items[:i], g.items[i+1:]...)
				}
			}
		}

		for i, ba := range g.beamAnimations {
			ba.Update()
			if ba.Step >= ba.Steps && i < len(g.beamAnimations) {
				g.beamAnimations = slices.Delete(g.beamAnimations, i, i+1)
			}
		}

		for i, a := range g.animations {
			a.Update()
			if a.currF >= a.numFrames && i < len(g.animations) && !a.looping {
				g.animations = slices.Delete(g.animations, i, i+1)
			}
			if a.name == "shield" && g.player.shield == nil && i < len(g.animations) {
				g.animations = slices.Delete(g.animations, i, i+1)
			}
		}

		// Remove blows
		for k, b := range g.blows {
			b.Update()
			if b.Step >= b.Steps && k < len(g.blows) {
				g.blows = slices.Delete(g.blows, k, k+1)
			}
		}

		// Remove beams
		for k, b := range g.beams {
			b.Update()
			if b.Step >= b.Steps && k < len(g.beams) {
				g.beams = slices.Delete(g.beams, k, k+1)
			}
		}
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	switch g.state {
	case config.Options:
		g.optionsMenu.Draw(screen)
	case config.Profile:
		g.profile.Draw(screen)
	case config.MainMenu:
		g.menu.Draw(screen)
	case config.InGame:
		g.DrawBg(screen)

		g.player.Draw(screen)

		for _, ba := range g.beamAnimations {
			ba.Draw(screen)
		}

		for _, e := range g.enemies {
			e.Draw(screen)
		}

		for _, m := range g.meteors {
			m.Draw(screen)
		}

		for _, p := range g.projectiles {
			p.Draw(screen)
		}

		for _, p := range g.enemyProjectiles {
			p.Draw(screen)
		}

		for _, i := range g.items {
			i.Draw(screen)
		}

		for i, a := range g.animations {
			if i < len(g.animations) {
				a.Draw(screen)
			}
		}

		// Draw the hit points bar
		barX := g.Options.ScreenWidth - 120*g.Options.ResolutionMultipler
		backWidth := float32(g.player.params.HP)*10 + 4
		if backWidth < 104*float32(g.Options.ResolutionMultipler) {
			backWidth = 104
		}

		vector.DrawFilledRect(screen, float32(barX-2), 38, backWidth, 24, color.RGBA{255, 255, 255, 255}, false)
		vector.DrawFilledRect(screen, float32(barX), 40, float32(g.player.params.HP)*10*float32(g.Options.ResolutionMultipler), 20, color.RGBA{179, 14, 14, 255}, false)

		// Draw shield bar
		if g.player.shield != nil {
			backWidth := float32(g.player.shield.HP)*10 + 4
			if backWidth < 104 {
				backWidth = 104
			}
			shiftX := float64(backWidth) - 104
			vector.DrawFilledRect(screen, float32(barX-shiftX-2), 82, backWidth, 24, color.RGBA{255, 255, 255, 255}, false)
			vector.DrawFilledRect(screen, float32(barX-shiftX), 84, float32(g.player.shield.HP)*10*float32(g.Options.ResolutionMultipler), 20, color.RGBA{26, 14, 189, 255}, false)
		}

		// Draw weapons
		for i, w := range g.player.weapons {
			offset := 20 * int(g.Options.ResolutionMultipler)
			object := w.projectile.wType.Sprite
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(i*offset+offset), g.Options.ScreenHeight-float64(60*int(g.Options.ResolutionMultipler)))
			if w.projectile.wType.WeaponName == g.player.curWeapon.projectile.wType.WeaponName {
				vector.DrawFilledRect(screen, float32(i*offset+offset-2), float32(g.Options.ScreenHeight-float64(30*int(g.Options.ResolutionMultipler))), float32(object.Bounds().Dx()+4*int(g.Options.ResolutionMultipler)), 3*float32(g.Options.ResolutionMultipler), color.RGBA{255, 255, 255, 255}, false)
			}
			text.Draw(screen, fmt.Sprintf("%v", w.ammo), g.Options.SmallFont, i*offset+offset, int(g.Options.ScreenHeight)-80*int(g.Options.ResolutionMultipler), color.White)
			screen.DrawImage(object, op)
		}

		// Draw secondary weapons
		for i, w := range g.player.secondaryWeapons {
			startOffset := int(g.Options.ScreenWidth) - 40*int(g.Options.ResolutionMultipler)
			offset := 20 * int(g.Options.ResolutionMultipler)
			object := w.projectile.wType.Sprite
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(startOffset-i*offset), g.Options.ScreenHeight-float64(60*int(g.Options.ResolutionMultipler)))
			if g.player.curSecondaryWeapon != nil && w.projectile.wType.WeaponName == g.player.curSecondaryWeapon.projectile.wType.WeaponName {
				vector.DrawFilledRect(screen, float32(startOffset-i*offset-2), float32(g.Options.ScreenHeight-float64(30*float32(g.Options.ResolutionMultipler))), float32(object.Bounds().Dx()+4*int(g.Options.ResolutionMultipler)), 3*float32(g.Options.ResolutionMultipler), color.RGBA{255, 255, 255, 255}, false)
			}
			text.Draw(screen, fmt.Sprintf("%v", w.ammo), g.Options.SmallFont, startOffset-i*offset, int(g.Options.ScreenHeight)-80*int(g.Options.ResolutionMultipler), color.White)
			screen.DrawImage(object, op)
		}

		// for _, a := range g.animations {
		// 	if a.name == "engineFireburst" {
		// 		msg := fmt.Sprintf("X: %v, Y: %v, Angle: %v, Step: %v, Frame: %v", a.position.X, a.position.Y, a.rotation, a.currF, a.curTick)
		// 		ebitenutil.DebugPrint(screen, msg)
		// 	}
		// }

		text.Draw(screen, fmt.Sprintf("Level: %v Stage: %v Wave: %v", g.curLevel.LevelId+1, g.CurStage.StageId+1, g.CurWave.WaveId), g.Options.InfoFont, 20, 50, color.White)
		text.Draw(screen, fmt.Sprintf("%06d", g.score), g.Options.ScoreFont, int(g.Options.ScreenWidth)/2-100, 50, color.White)
	}
}

func (g *Game) AddProjectile(p *Projectile) {
	if p.owner == "player" {
		g.projectiles = append(g.projectiles, p)
	} else {
		g.enemyProjectiles = append(g.enemyProjectiles, p)
	}
}

func (g *Game) AddBeam(b *Beam) {
	if b.owner == "player" {
		g.beams = append(g.beams, b)
	} else {
		g.enemyBeams = append(g.enemyBeams, b)
	}
}

func (g *Game) AddBeamAnimation(b *BeamAnimation) {
	g.beamAnimations = append(g.beamAnimations, b)
}

func (g *Game) AddAnimation(a *Animation) {
	g.animations = append(g.animations, a)
}

func (g *Game) AddBlow(b *Blow, target config.Vector) {
	g.blows = append(g.blows, b)
	blowAnimation := NewAnimation(target, assets.BigBlowSpriteSheet, 1, 124, 128, false, "digBlow", 0)
	g.AddAnimation(blowAnimation)
}

func (g *Game) KillEnemy(i int) {
	enemyBlow := NewAnimation(g.enemies[i].position, assets.EnemyBlowSpriteSheet, 1, 73, 75, false, "enemyBlow", 0)
	g.AddAnimation(enemyBlow)
	g.enemies = append(g.enemies[:i], g.enemies[i+1:]...)
	g.score++
	g.profile.credits += 10
}

func (g *Game) IntersectProjectile(curPr *Projectile, i int) {
	curPr.intercectAnimation.position = curPr.position
	curPr.AddAnimation(g)
	curPr.HP--
	if curPr.HP <= 0 {
		curPr.Destroy(g, i)
	}
}

func (g *Game) Reset() {
	g.meteors = nil
	g.projectiles = nil
	g.enemyProjectiles = nil
	g.enemies = nil
	g.items = nil
	g.beams = nil
	g.beamAnimations = nil
	g.animations = nil
	g.score = 0
	g.player = NewPlayer(g)
	var newLevels = config.NewLevels()
	g.curLevel = newLevels[0]
	g.CurStage = &g.curLevel.Stages[0]
	g.CurWave = &g.CurStage.Waves[0]
	g.levels = newLevels
	g.meteorSpawnTimer.Reset()
	g.batchesSpawnTimer = config.NewTimer(g.levels[0].Stages[0].Waves[0].Batches[0].BatchSpawnTime)
	g.itemSpawnTimer.Reset()
	g.baseVelocity = config.BaseMeteorVelocity
	g.velocityTimer.Reset()
	g.started = false
	g.menu = NewMainMenu(g)
	g.profile = NewPlayerProfile(g)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return int(g.Options.ScreenWidth), int(g.Options.ScreenHeight)
}

func (g *Game) DrawBg(screen *ebiten.Image) {
	_, y16 := g.BgPosition()
	offsetY := float64(-y16) / 10

	// Draw bgImage on the screen repeatedly.
	const repeat = 10
	h := g.bgImage.Bounds().Dy()
	for j := 0; j < repeat; j++ {
		for i := 0; i < repeat; i++ {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(0, -float64(h*j))
			op.GeoM.Translate(0, -offsetY)
			screen.DrawImage(g.bgImage, op)
		}
	}
}
