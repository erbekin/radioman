package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type VisualizerMode int

const (
	VizModeSpectrum VisualizerMode = iota
	VizModeMirror
	VizModeWave
)

func (m VisualizerMode) String() string {
	switch m {
	case VizModeSpectrum:
		return "Spectrum Bars"
	case VizModeMirror:
		return "Mirror EQ"
	case VizModeWave:
		return "Waveform"
	default:
		return "Spectrum"
	}
}

type Visualizer struct {
	Enabled bool
	Mode    VisualizerMode

	levels [64]float64
	peaks  [64]float64
	phase  float64
}

var (
	vizBarChars = []rune{' ', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	vizRowColors = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Bold(true), // Top peak (red/pink)
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")).Bold(true), // High (yellow)
		lipgloss.NewStyle().Foreground(lipgloss.Color("#CBA6F7")),            // Upper-mid (purple)
		lipgloss.NewStyle().Foreground(lipgloss.Color("#89B4FA")),            // Mid (blue)
		lipgloss.NewStyle().Foreground(lipgloss.Color("#89DCEB")),            // Lower-mid (cyan)
		lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")),            // Low (green)
	}

	vizPeakStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F5E0DC")).Bold(true)
)

func NewVisualizer(enabled bool) *Visualizer {
	return &Visualizer{
		Enabled: enabled,
		Mode:    VizModeSpectrum,
	}
}

func (v *Visualizer) Toggle() bool {
	v.Enabled = !v.Enabled
	return v.Enabled
}

func (v *Visualizer) NextMode() VisualizerMode {
	v.Mode = (v.Mode + 1) % 3
	return v.Mode
}

func (v *Visualizer) Step(playing bool, streamBands [32]float64, hasStreamSignal bool) {
	if !playing {
		for i := range v.levels {
			v.levels[i] *= 0.72
			if v.levels[i] < 0.01 {
				v.levels[i] = 0
			}
			v.peaks[i] *= 0.85
			if v.peaks[i] < 0.01 {
				v.peaks[i] = 0
			}
		}
		return
	}

	v.phase += 0.22
	now := float64(time.Now().UnixMilli()) / 1000.0

	for i := range v.levels {
		norm := float64(i) / float64(len(v.levels))
		srcIdx := min(31, int(norm*32))

		var target float64
		if hasStreamSignal {
			target = streamBands[srcIdx]
			// Add subtle natural motion around the measured stream energy
			wobble := 0.08 * math.Sin(v.phase*1.7+float64(i)*0.45)
			target = clamp01(target + wobble)
		} else {
			// Smooth procedural audio-like spectrum if stream bytes aren't available yet
			bass := math.Exp(-norm*2.5) * (0.55 + 0.40*math.Sin(now*4.2+float64(i)*0.2))
			mid := math.Exp(-math.Pow((norm-0.45)*3.2, 2)) * (0.45 + 0.35*math.Sin(now*6.5-float64(i)*0.35))
			treble := math.Exp(-math.Pow((norm-0.8)*4.0, 2)) * (0.30 + 0.25*math.Cos(now*9.1+float64(i)*0.6))
			beat := 0.15 * math.Pow(0.5+0.5*math.Sin(now*3.14159*2.1), 3)
			target = clamp01(bass*0.55 + mid*0.5 + treble*0.45 + beat)
		}

		// Fast attack, smooth decay
		if target > v.levels[i] {
			v.levels[i] = v.levels[i]*0.35 + target*0.65
		} else {
			v.levels[i] = v.levels[i]*0.78 + target*0.22
		}

		if v.levels[i] >= v.peaks[i] {
			v.peaks[i] = v.levels[i]
		} else {
			v.peaks[i] = max(0, v.peaks[i]-0.035)
		}
	}
}

func (v *Visualizer) Render(width, height int, playing bool) string {
	if width < 10 || height < 3 {
		return ""
	}

	header := fmt.Sprintf(
		"%s %s",
		labelStyle.Render("♫ Visualizer:"),
		dimStyle.Render(fmt.Sprintf("[%s • 'v': toggle • 'm': mode]", v.Mode.String())),
	)

	plotHeight := max(2, height-2)
	var body string
	switch v.Mode {
	case VizModeMirror:
		body = v.renderMirror(width, plotHeight)
	case VizModeWave:
		body = v.renderWave(width, plotHeight, playing)
	default:
		body = v.renderSpectrum(width, plotHeight)
	}

	return header + "\n" + body
}

func (v *Visualizer) renderSpectrum(width, height int) string {
	numBars := min(len(v.levels), max(8, width/2))
	rows := make([]string, height)

	for r := range height {
		// r=0 is top row, r=height-1 is bottom row
		rowBottom := float64(height-1-r) / float64(height)
		rowTop := float64(height-r) / float64(height)

		colorIdx := int(float64(r) / float64(max(1, height)) * float64(len(vizRowColors)))
		colorIdx = min(len(vizRowColors)-1, max(0, colorIdx))
		rowStyle := vizRowColors[colorIdx]

		var sb strings.Builder
		for b := range numBars {
			idx := b * len(v.levels) / numBars
			lvl := v.levels[idx]
			peak := v.peaks[idx]

			var cell string
			if lvl >= rowTop {
				cell = rowStyle.Render("█")
			} else if lvl > rowBottom {
				frac := (lvl - rowBottom) / (rowTop - rowBottom)
				charIdx := int(math.Round(frac * float64(len(vizBarChars)-1)))
				charIdx = min(len(vizBarChars)-1, max(1, charIdx))
				cell = rowStyle.Render(string(vizBarChars[charIdx]))
			} else if peak >= rowBottom && peak <= rowTop && peak > 0.05 {
				cell = vizPeakStyle.Render("▔")
			} else {
				cell = " "
			}

			sb.WriteString(cell)
			if b < numBars-1 {
				sb.WriteByte(' ')
			}
		}
		rows[r] = sb.String()
	}

	return strings.Join(rows, "\n")
}

func (v *Visualizer) renderMirror(width, height int) string {
	numBars := min(len(v.levels), max(8, width/2))
	rows := make([]string, height)
	mid := float64(height-1) / 2.0
	halfSpan := max(1.0, float64(height)/2.0)

	for r := range height {
		dist := math.Abs(float64(r)-mid) / halfSpan
		colorIdx := int((1.0 - dist) * float64(len(vizRowColors)-1))
		colorIdx = min(len(vizRowColors)-1, max(0, colorIdx))
		rowStyle := vizRowColors[colorIdx]

		var sb strings.Builder
		for b := range numBars {
			idx := b * len(v.levels) / numBars
			lvl := v.levels[idx]

			if lvl >= dist {
				if dist < 0.18 {
					sb.WriteString(rowStyle.Render("█"))
				} else if lvl-dist < 0.15 {
					sb.WriteString(rowStyle.Render("▒"))
				} else {
					sb.WriteString(rowStyle.Render("▓"))
				}
			} else {
				sb.WriteByte(' ')
			}
			if b < numBars-1 {
				sb.WriteByte(' ')
			}
		}
		rows[r] = sb.String()
	}

	return strings.Join(rows, "\n")
}

func (v *Visualizer) renderWave(width, height int, playing bool) string {
	cols := min(width, 72)
	grid := make([][]rune, height)
	for r := range height {
		grid[r] = make([]rune, cols)
		for c := range cols {
			grid[r][c] = ' '
		}
	}

	mid := float64(height-1) / 2.0
	for c := range cols {
		idx := c * len(v.levels) / max(1, cols)
		amp := v.levels[idx]
		if !playing {
			amp = 0.02
		}
		x := float64(c) / float64(max(1, cols))
		y := math.Sin(x*math.Pi*4.0+v.phase)*0.65 + math.Sin(x*math.Pi*9.0-v.phase*1.4)*0.35
		rowFloat := mid - y*amp*mid
		r := int(math.Round(rowFloat))
		r = min(height-1, max(0, r))
		grid[r][c] = '•'
		if r+1 < height && math.Abs(rowFloat-float64(r)) > 0.35 {
			grid[r+1][c] = '·'
		}
	}

	rows := make([]string, height)
	for r := range height {
		colorIdx := int(float64(r) / float64(max(1, height)) * float64(len(vizRowColors)))
		colorIdx = min(len(vizRowColors)-1, max(0, colorIdx))
		rows[r] = vizRowColors[colorIdx].Render(string(grid[r]))
	}
	return strings.Join(rows, "\n")
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
