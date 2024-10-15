package main

import (
	"os/exec"
	"strings"
	"time"

	_ "embed"

	"github.com/avamsi/climate"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/shlex"
	"github.com/mattn/go-runewidth"
)

type parsedRune struct {
	rune
	visible bool
	width   int
}

type parsedLine struct {
	runes []parsedRune
	width int
}

func (pl *parsedLine) write(r rune, visible bool) {
	var (
		width = runewidth.RuneWidth(r)
		pr    = parsedRune{rune: r, visible: visible, width: width}
	)
	pl.runes = append(pl.runes, pr)
	if visible {
		pl.width += width
	}
}

func parseLine(s string) parsedLine {
	var (
		runes      = []rune(strings.ReplaceAll(s, "\t", "    "))
		pl         = parsedLine{runes: make([]parsedRune, 0, len(runes))}
		visible    = true
		terminator []rune
	)
	for i := 0; i < len(runes); i++ {
		// Sloppy, but probably good enough for now: color codes end in `m`
		// and hyperlinks in `ESC\` (at least, the ones I use).
		if visible && runes[i] == 0x1b && i+1 < len(runes) {
			switch runes[i+1] {
			case '[':
				visible = false
				terminator = []rune{'m'}
			case ']':
				visible = false
				terminator = []rune{'\x1b', '\\'}
			}
		}
		pl.write(runes[i], visible)
		if !visible && runes[i] == terminator[0] {
			switch len(terminator) {
			case 1:
				visible = true
			case 2:
				if i+1 < len(runes) && runes[i+1] == terminator[1] {
					pl.write(runes[i+1], false)
					visible = true
					i++
				}
			}
		}
	}
	return pl
}

type parsedText struct {
	lines []parsedLine
	width int
}

func parseText(s string) parsedText {
	var (
		lines = strings.Split(s, "\n")
		pt    = parsedText{lines: make([]parsedLine, len(lines))}
	)
	for i, line := range lines {
		pt.lines[i] = parseLine(line)
		pt.width = max(pt.width, pt.lines[i].width)
	}
	return pt
}

func (pt parsedText) viewport(x, y, width, height int) string {
	var (
		b  strings.Builder
		x2 = min(x+width, pt.width)
		x1 = max(x2-width, 0)
		y2 = min(y+height, len(pt.lines))
		y1 = max(y2-height, 0)
	)
	b.Grow((x2 - x1) * (y2 - y1))
	for i, line := range pt.lines[y1:y2] {
		if i > 0 {
			b.WriteByte('\n')
		}
		x, width := x1, width
		for _, pr := range line.runes {
			if pr.visible {
				if x > 0 {
					x -= pr.width
					continue
				}
				if width > 0 {
					width -= pr.width
				} else {
					b.WriteString("\x1b[0m")
					break
				}
			}
			b.WriteRune(pr.rune)
		}
	}
	return b.String()
}

type model struct {
	d   time.Duration
	cmd []string

	t             time.Time
	output        parsedText
	x, y          int
	width, height int
}

type outputMsg string

func (m model) execute() tea.Cmd {
	return func() tea.Msg {
		var (
			cmd         = exec.Command(m.cmd[0], m.cmd[1:]...)
			output, err = cmd.CombinedOutput()
		)
		if err != nil {
			return outputMsg(err.Error())
		}
		return outputMsg(output)
	}
}

func (m model) Init() tea.Cmd {
	return m.execute()
}

type tickMsg time.Time

func (m model) tick() tea.Cmd {
	return tea.Tick(m.d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case outputMsg:
		m.output = parseText(string(msg))
		return m, m.tick()
	case tickMsg:
		m.t = time.Time(msg)
		return m, m.execute()
	case tea.WindowSizeMsg:
		m.x, m.y = 0, 0
		m.width, m.height = msg.Width, msg.Height
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.y = max(m.y-1, 0)
		case tea.MouseButtonWheelRight:
			m.x = min(m.x+1, m.width-1)
		case tea.MouseButtonWheelDown:
			m.y = min(m.y+1, m.height-1)
		case tea.MouseButtonWheelLeft:
			m.x = max(m.x-1, 0)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "up":
			m.y = max(m.y-1, 0)
		case "right":
			m.x = min(m.x+1, m.width-1)
		case "down":
			m.y = min(m.y+1, m.height-1)
		case "left":
			m.x = max(m.x-1, 0)
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func boxView(title, content string, width int) string {
	border := lipgloss.NormalBorder()
	border.Top = title + strings.Repeat(border.Top, width-len(title))
	style := lipgloss.NewStyle().
		Border(border).BorderForeground(lipgloss.Color("59")).Width(width)
	return style.Render(content)
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	var (
		headerView = lipgloss.JoinHorizontal(
			lipgloss.Top,
			boxView("Every", m.d.String(), 8),
			boxView("Command", strings.Join(m.cmd, " "), m.width-33),
			boxView("Time", m.t.Format(time.DateTime), 19),
		)
		remainingHeight = m.height - lipgloss.Height(headerView)
		commandView     = m.output.viewport(m.x, m.y, m.width, remainingHeight)
	)
	return headerView + "\n" + commandView
}

// dekh is a simple modern alternative to the watch command.
func dekh(cmd []string) error {
	switch len(cmd) {
	case 0:
		return nil
	case 1:
		var err error
		if cmd, err = shlex.Split(cmd[0]); err != nil {
			return err
		}
	}
	var (
		m      = model{d: 2 * time.Second, cmd: cmd}
		p      = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
		_, err = p.Run()
	)
	return err
}

//go:generate go run github.com/avamsi/climate/cmd/cligen --out=md.cli
//go:embed md.cli
var md []byte

func main() {
	climate.RunAndExit(climate.Func(dekh), climate.WithMetadata(md))
}
