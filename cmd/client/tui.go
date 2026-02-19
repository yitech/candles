package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	pb "github.com/yitech/candles/model/protobuf"
)

// ── styles ────────────────────────────────────────────────────────────────────

var (
	bullStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#26a641"))
	bearStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#e05c5c"))
	wickStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	axisStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#aaaaaa"))
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
)

// ── messages ──────────────────────────────────────────────────────────────────

type candleMsg struct{ c *pb.Candle }

// ── model ─────────────────────────────────────────────────────────────────────

type model struct {
	symbol   string
	interval string
	nKline   int
	ch       <-chan *pb.Candle

	candles []*pb.Candle
	width   int
	height  int
}

func newModel(symbol, interval string, nKline int, ch <-chan *pb.Candle) model {
	return model{
		symbol:   symbol,
		interval: interval,
		nKline:   nKline,
		ch:       ch,
	}
}

// ── Init / Update / View ──────────────────────────────────────────────────────

func (m model) Init() tea.Cmd {
	return waitForCandle(m.ch)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case candleMsg:
		m.addOrUpdate(msg.c)
		return m, waitForCandle(m.ch)
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		return "connecting…"
	}
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteByte('\n')
	b.WriteString(m.renderChart())
	b.WriteByte('\n')
	b.WriteString(footerStyle.Render("[q] quit"))
	return b.String()
}

// ── helpers ───────────────────────────────────────────────────────────────────

// waitForCandle blocks on the channel and returns a Cmd that fires candleMsg.
func waitForCandle(ch <-chan *pb.Candle) tea.Cmd {
	return func() tea.Msg {
		return candleMsg{<-ch}
	}
}

// addOrUpdate merges into the last candle if openTime matches, else appends.
func (m *model) addOrUpdate(c *pb.Candle) {
	if n := len(m.candles); n > 0 && m.candles[n-1].OpenTime == c.OpenTime {
		m.candles[n-1] = c
	} else {
		m.candles = append(m.candles, c)
		if len(m.candles) > m.nKline {
			m.candles = m.candles[len(m.candles)-m.nKline:]
		}
	}
}

// ── header ────────────────────────────────────────────────────────────────────

func (m model) renderHeader() string {
	if len(m.candles) == 0 {
		return headerStyle.Render(fmt.Sprintf("%s  %s  waiting for data…", m.symbol, m.interval))
	}
	c := m.candles[len(m.candles)-1]
	status := "open"
	if c.IsClosed {
		status = "closed"
	}
	return headerStyle.Render(fmt.Sprintf(
		"%s  %s  [%s]  O:%s  H:%s  L:%s  C:%s  V:%s  %d/%d",
		m.symbol, m.interval, status,
		c.Open, c.High, c.Low, c.Close, c.Volume,
		len(m.candles), m.nKline,
	))
}

// ── chart ─────────────────────────────────────────────────────────────────────

const yAxisWidth = 11 // "  12345.67 │"

func (m model) renderChart() string {
	// Reserve: 1 header + chart rows + 1 x-axis line + 1 time-label line + 1 footer
	chartH := m.height - 4
	if chartH < 3 {
		chartH = 3
	}

	candles := m.candles
	chartW := m.width - yAxisWidth
	maxCols := chartW / 2 // each candle occupies 2 chars
	if maxCols < 1 {
		maxCols = 1
	}
	if len(candles) > maxCols {
		candles = candles[len(candles)-maxCols:]
	}

	// Price range across visible candles.
	hi, lo := priceRange(candles)
	if hi == lo {
		hi = lo + 1
	}

	// Build a 2-D grid: rows × cols of strings (each cell is one char, styled).
	cols := len(candles) * 2
	grid := make([][]string, chartH)
	for r := range grid {
		grid[r] = make([]string, cols)
		for c := range grid[r] {
			grid[r][c] = " "
		}
	}

	for i, c := range candles {
		renderCandle(grid, c, i*2, chartH, hi, lo)
	}

	// Render rows with Y-axis labels.
	var b strings.Builder
	for row := 0; row < chartH; row++ {
		price := rowToPrice(row, chartH, hi, lo)
		label := fmt.Sprintf("%9.2f │", price)
		b.WriteString(axisStyle.Render(label))
		b.WriteString(strings.Join(grid[row], ""))
		b.WriteByte('\n')
	}

	// X-axis separator.
	b.WriteString(axisStyle.Render(strings.Repeat("─", yAxisWidth)))
	b.WriteString(axisStyle.Render(strings.Repeat("─", cols)))
	b.WriteByte('\n')

	// Time labels — one timestamp per ~10 candles.
	b.WriteString(strings.Repeat(" ", yAxisWidth))
	labelEvery := 10
	for i, c := range candles {
		cell := "  "
		if i%labelEvery == 0 {
			t := time.UnixMilli(c.OpenTime).UTC()
			cell = t.Format("15:04")
			// pad / trim to exactly 2 chars (only first 2 of "15:04" is 5 chars,
			// so we print across multiple cells — just anchor at position i)
			b.WriteString(cell)
			// Skip the next 4 columns (5-char timestamp spread across cells).
			skip := 4
			if i+skip/2 >= len(candles) {
				skip = (len(candles) - i - 1) * 2
			}
			// advance i by skip chars — handled by loop, so write spaces
			_ = skip
			continue
		}
		b.WriteString(cell)
	}
	b.WriteByte('\n')

	return b.String()
}

// renderCandle paints one candle into the grid at column x (0-indexed, 2 wide).
func renderCandle(grid [][]string, c *pb.Candle, x, chartH int, hi, lo float64) {
	open, _  := strconv.ParseFloat(c.Open,  64)
	cls, _   := strconv.ParseFloat(c.Close, 64)
	high, _  := strconv.ParseFloat(c.High,  64)
	low, _   := strconv.ParseFloat(c.Low,   64)

	bullish := cls >= open
	style   := bullStyle
	if !bullish {
		style = bearStyle
	}

	fH := float64(chartH)
	bodyTop := priceToRow(math.Max(open, cls), fH, hi, lo)
	bodyBot := priceToRow(math.Min(open, cls), fH, hi, lo)
	wickTop := priceToRow(high, fH, hi, lo)
	wickBot := priceToRow(low,  fH, hi, lo)

	for row := 0; row < chartH; row++ {
		inBody := row >= bodyTop && row <= bodyBot
		inWick := row >= wickTop && row <= wickBot

		var left, right string
		switch {
		case inBody:
			left  = style.Render("█")
			right = style.Render("█")
		case inWick:
			left  = wickStyle.Render("│")
			right = " "
		default:
			left  = " "
			right = " "
		}

		if x < len(grid[row]) {
			grid[row][x] = left
		}
		if x+1 < len(grid[row]) {
			grid[row][x+1] = right
		}
	}
}

// priceToRow converts a price to a grid row (0 = top = high).
func priceToRow(price, chartH float64, hi, lo float64) int {
	if hi == lo {
		return int(chartH) / 2
	}
	row := (hi - price) / (hi - lo) * (chartH - 1)
	r := int(math.Round(row))
	if r < 0 {
		r = 0
	}
	if r >= int(chartH) {
		r = int(chartH) - 1
	}
	return r
}

// rowToPrice is the inverse of priceToRow.
func rowToPrice(row, chartH int, hi, lo float64) float64 {
	if chartH <= 1 {
		return hi
	}
	return hi - float64(row)/float64(chartH-1)*(hi-lo)
}

// priceRange returns the overall high and low across the visible candles.
func priceRange(candles []*pb.Candle) (hi, lo float64) {
	hi = -math.MaxFloat64
	lo = math.MaxFloat64
	for _, c := range candles {
		if h, err := strconv.ParseFloat(c.High, 64); err == nil && h > hi {
			hi = h
		}
		if l, err := strconv.ParseFloat(c.Low, 64); err == nil && l < lo {
			lo = l
		}
	}
	if hi == -math.MaxFloat64 {
		hi = 0
	}
	if lo == math.MaxFloat64 {
		lo = 0
	}
	return
}
