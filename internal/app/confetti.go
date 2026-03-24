package app

import (
	"image/color"
	"math/rand/v2"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

const (
	confettiParticles = 20
	confettiFrames    = 22
	confettiInterval  = 50 * time.Millisecond
	necklaceCount     = 5 // number of bead necklaces
	necklaceLength    = 5 // beads per necklace
)

var confettiGlyphs = []string{"●", "◆", "⚜", "✦", "✧", "★", "♦"}
var necklaceGlyphs = []string{"●", "◆", "●", "◆", "●"} // alternating bead shapes
var confettiColors = []color.Color{
	ui.Purple, ui.Gold, ui.Green,
	ui.BrightPurple, ui.BrightGold, ui.BrightGreen,
}

type particle struct {
	x, y   float64
	vx, vy float64
	glyph  string
	color  color.Color
	styled string // pre-rendered styled glyph
}

// necklace is a vertical chain of connected beads that falls together.
type necklace struct {
	x            float64       // horizontal position
	y            float64       // top bead position
	vy           float64       // vertical velocity
	vx           float64       // slight horizontal sway
	beads        []color.Color // color per bead
	glyphs       []string      // glyph per bead
	styledBeads  []string      // pre-rendered styled beads
	styledConns  []string      // pre-rendered styled connectors
}

// Confetti is a particle animation triggered on issue close.
// Combines scattered particles with falling bead necklaces.
type Confetti struct {
	particles []particle
	necklaces []necklace
	frame     int
	width     int
	height    int
	active    bool
}

// confettiTickMsg advances the animation one frame.
type confettiTickMsg struct{}

// NewConfetti creates a confetti animation centered on the screen.
// Includes both scattered particles and falling bead necklaces.
func NewConfetti(width, height int) Confetti {
	centerX := float64(width) / 2
	centerY := float64(height) / 2

	particles := make([]particle, confettiParticles)
	for i := range particles {
		g := confettiGlyphs[rand.IntN(len(confettiGlyphs))]
		c := confettiColors[rand.IntN(len(confettiColors))]
		particles[i] = particle{
			x:      centerX,
			y:      centerY,
			vx:     (rand.Float64() - 0.5) * 6,
			vy:     (rand.Float64() - 0.8) * 5, // bias upward
			glyph:  g,
			color:  c,
			styled: lipgloss.NewStyle().Foreground(c).Render(g),
		}
	}

	// Create bead necklaces that fall from the top
	necklaces := make([]necklace, necklaceCount)
	for i := range necklaces {
		beadColors := make([]color.Color, necklaceLength)
		beadGlyphs := make([]string, necklaceLength)
		styledBeads := make([]string, necklaceLength)
		styledConns := make([]string, necklaceLength)

		// Each necklace uses a consistent Mardi Gras color trio
		baseIdx := i % 3
		colorTriple := []color.Color{ui.BrightPurple, ui.BrightGold, ui.BrightGreen}
		for j := range beadColors {
			c := colorTriple[(baseIdx+j)%3]
			g := necklaceGlyphs[j%len(necklaceGlyphs)]
			beadColors[j] = c
			beadGlyphs[j] = g
			styledBeads[j] = lipgloss.NewStyle().Foreground(c).Render(g)
			styledConns[j] = lipgloss.NewStyle().Foreground(c).Render("│")
		}
		necklaces[i] = necklace{
			x:           centerX + (rand.Float64()-0.5)*float64(width)*0.6,
			y:           -float64(necklaceLength) - rand.Float64()*3, // start above screen
			vy:          0.8 + rand.Float64()*0.4,                    // gentle fall
			vx:          (rand.Float64() - 0.5) * 0.3,                // slight sway
			beads:       beadColors,
			glyphs:      beadGlyphs,
			styledBeads: styledBeads,
			styledConns: styledConns,
		}
	}

	return Confetti{
		particles: particles,
		necklaces: necklaces,
		frame:     0,
		width:     width,
		height:    height,
		active:    true,
	}
}

// Tick returns a command to advance the animation.
func (c Confetti) Tick() tea.Cmd {
	if !c.active {
		return nil
	}
	return tea.Tick(confettiInterval, func(time.Time) tea.Msg {
		return confettiTickMsg{}
	})
}

// Update advances particle positions by one frame.
func (c *Confetti) Update() {
	if !c.active {
		return
	}
	c.frame++
	if c.frame >= confettiFrames {
		c.active = false
		return
	}

	gravity := 0.3
	for i := range c.particles {
		c.particles[i].x += c.particles[i].vx
		c.particles[i].y += c.particles[i].vy
		c.particles[i].vy += gravity // gravity pulls down
		c.particles[i].vx *= 0.95    // slow horizontal movement
	}

	// Update necklaces: gentle fall with slight sway
	for i := range c.necklaces {
		c.necklaces[i].y += c.necklaces[i].vy
		c.necklaces[i].x += c.necklaces[i].vx
		c.necklaces[i].vy += 0.05 // gentle gravity
	}
}

// View renders the confetti overlay. Returns empty string if not active.
func (c Confetti) View() string {
	if !c.active || c.width == 0 || c.height == 0 {
		return ""
	}

	// Build a character grid
	grid := make([][]string, c.height)
	for y := range grid {
		grid[y] = make([]string, c.width)
		for x := range grid[y] {
			grid[y][x] = " "
		}
	}

	// Place particles
	for _, p := range c.particles {
		px := int(p.x)
		py := int(p.y)
		if px >= 0 && px < c.width && py >= 0 && py < c.height {
			grid[py][px] = p.styled
		}
	}

	// Place necklaces: vertical chains of beads connected by │
	for _, n := range c.necklaces {
		px := int(n.x)
		if px < 0 || px >= c.width {
			continue
		}
		for j := range n.beads {
			// Each bead is at y + j*2 (bead, connector, bead, connector...)
			beadY := int(n.y) + j*2
			if beadY >= 0 && beadY < c.height {
				grid[beadY][px] = n.styledBeads[j]
			}
			// Connector between beads
			connY := beadY + 1
			if j < len(n.beads)-1 && connY >= 0 && connY < c.height {
				grid[connY][px] = n.styledConns[j]
			}
		}
	}

	// Render grid
	var lines []string
	for y := range grid {
		var line strings.Builder
		for x := range grid[y] {
			line.WriteString(grid[y][x])
		}
		lines = append(lines, line.String())
	}

	return strings.Join(lines, "\n")
}

// Active returns whether the animation is still running.
func (c Confetti) Active() bool {
	return c.active
}
