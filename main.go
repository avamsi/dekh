package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	_ "embed"

	"github.com/avamsi/climate"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/shlex"
)

type model struct {
	d   time.Duration
	cmd []string

	t      time.Time
	output string
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
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	return fmt.Sprintf(
		"Every: %s\tCommand: %s\tTime: %s\n\n%s",
		m.d, strings.Join(m.cmd, " "), m.t.Format(time.DateTime), m.output)
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
		p      = tea.NewProgram(m, tea.WithAltScreen())
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
