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
	var b strings.Builder
	width = min(width, pt.width)
	height = min(height, len(pt.lines))
	b.Grow(width * height)
	for i, line := range pt.lines[y : y+height] {
		if i > 0 {
			b.WriteByte('\n')
		}
		x, width := x, width
		for _, pr := range line.runes {
			switch {
			case !pr.visible:
				b.WriteRune(pr.rune)
			case x > 0:
				x -= pr.width
			case width > 0:
				width -= pr.width
				b.WriteRune(pr.rune)
			default:
				b.WriteString("\x1b[0m")
				break
			}
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

func (m model) snapped() model {
	var (
		x2 = min(m.x+m.width, m.output.width)
		x1 = max(x2-m.width, 0)
		y2 = min(m.y+m.height, len(m.output.lines))
		y1 = max(y2-m.height, 0)
	)
	m.x, m.y = x1, y1
	return m
}

type tickMsg time.Time

func (m model) tick() tea.Cmd {
	return tea.Tick(m.d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func boxView(title, content string, width int) string {
	var (
		border  = lipgloss.NormalBorder()
		padding = max(width-len(title), 0)
	)
	border.Top = title + strings.Repeat(border.Top, padding)
	style := lipgloss.NewStyle().
		Border(border).BorderForeground(lipgloss.Color("59")).Width(width)
	return style.Render(content)
}

func (m model) headerView() string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		boxView("Every", m.d.String(), 8),
		boxView("Command", strings.Join(m.cmd, " "), m.width-33),
		boxView("Time", m.t.Format(time.DateTime), 19))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case outputMsg:
		m.output = parseText(string(msg))
		return m.snapped(), m.tick()
	case tickMsg:
		m.t = time.Time(msg)
		return m, m.execute()
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.height -= lipgloss.Height(m.headerView())
	case tea.MouseMsg:
		switch msg.Button {
		// TODO: Should these also be w/h based?
		case tea.MouseButtonWheelUp:
			m.y -= 5
		case tea.MouseButtonWheelRight:
			m.x += 25
		case tea.MouseButtonWheelDown:
			m.y += 5
		case tea.MouseButtonWheelLeft:
			m.x -= 25
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "u":
			m.y -= m.height / 2
		case "d":
			m.y += m.height / 2
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m.snapped(), nil
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	return m.headerView() + "\n" +
		m.output.viewport(m.x, m.y, m.width, m.height)
}

// dekh is a simple, modern alternative to the watch command.
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
		m = model{d: 2 * time.Second, cmd: cmd}
		p = tea.NewProgram(m,
			tea.WithAltScreen(), tea.WithMouseCellMotion())
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
