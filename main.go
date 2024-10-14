package main

import (
	"fmt"
	"iter"
	"os/exec"
	"strings"
	"time"

	_ "embed"

	"github.com/avamsi/climate"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/shlex"
	"github.com/mattn/go-runewidth"
)

type model struct {
	d   time.Duration
	cmd []string

	t             time.Time
	output        string
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
		m.output = string(msg)
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

func maxWidth(lines []string) int {
	var n int
	for _, line := range lines {
		// This is obviously not correct, but eh.
		n = max(n, len(line))
	}
	return n
}

func parse(s string) iter.Seq2[rune, bool] {
	return func(yield func(rune, bool) bool) {
		var (
			s          = []rune(strings.ReplaceAll(s, "\t", "    "))
			visible    = true
			terminator []rune
		)
		for i := 0; i < len(s); i++ {
			// Sloppy, but probably good enough for now: color codes end in `m`
			// and hyperlinks in `\` (at least, the ones I use).
			if visible && s[i] == 0x1b && i+1 < len(s) {
				switch s[i+1] {
				case '[':
					terminator, visible = []rune{'m'}, false
				case ']':
					terminator, visible = []rune{'\x1b', '\\'}, false
				}
			}
			if !yield(s[i], visible) {
				break
			}
			if !visible && s[i] == terminator[0] {
				switch len(terminator) {
				case 1:
					visible = true
				case 2:
					if i+1 < len(s) && s[i+1] == terminator[1] {
						if !yield(s[i+1], false) {
							break
						}
						visible = true
						i++
					}
				}
			}
		}
	}
}

func viewport(s string, x, y, width, height int) string {
	var (
		lines = strings.Split(s, "\n")
		b     strings.Builder
		y2    = min(y+height, len(lines))
		y1    = max(y2-height, 0)
		x2    = min(x+width, maxWidth(lines))
		x1    = max(x2-width, 0)
	)
	b.Grow(len(s))
	for i, line := range lines[y1:y2] {
		if i > 0 {
			b.WriteByte('\n')
		}
		x, width := x1, width
		for r, visible := range parse(line) {
			if visible {
				if x > 0 {
					x -= runewidth.RuneWidth(r)
					continue
				}
				if width > 0 {
					width -= runewidth.RuneWidth(r)
				} else {
					b.WriteString("\x1b[0m")
					break
				}
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (m model) View() string {
	view := fmt.Sprintf(
		"Every: %s\tCommand: %s\tTime: %s\n\n%s",
		m.d, strings.Join(m.cmd, " "), m.t.Format(time.DateTime), m.output)
	return viewport(view, m.x, m.y, m.width, m.height)
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
