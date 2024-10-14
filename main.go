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

	t time.Time
}

func (m model) tick() tea.Cmd {
	return tea.Tick(m.d, func(t time.Time) tea.Msg {
		return t
	})
}

func (m model) Init() tea.Cmd {
	return m.tick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case time.Time:
		m.t = msg
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}
	}
	return m, m.tick()
}

func (m model) headerView() string {
	return fmt.Sprintf(
		"Every: %s\tCommand: %s\tTime: %s\n",
		m.d, strings.Join(m.cmd, " "), m.t.Format(time.DateTime))
}

func (m model) commandView() string {
	var (
		cmd         = exec.Command(m.cmd[0], m.cmd[1:]...)
		output, err = cmd.CombinedOutput()
	)
	if err != nil {
		return err.Error()
	}
	return string(output)
}

func (m model) View() string {
	return m.headerView() + "\n" + m.commandView()
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
