
package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type model struct {
	processes []ProcessIO
	selected  int
	showFiles bool
}

type ProcessIO struct {
	PID        int32
	Name       string
	ReadBytes  float64
	WriteBytes float64
	OpenFiles  []string
	CPUPercent float64
	MemPercent float32
}

func humanizeBytes(bytes float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	unitIndex := 0
	value := bytes

	for value > 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}

	return fmt.Sprintf("%.2f %s", value, units[unitIndex])
}

func getSystemStats() (float64, float64) {
	cpuPercent, err := cpu.Percent(0, false)
	if err != nil || len(cpuPercent) == 0 {
		return 0, 0
	}

	memStats, err := mem.VirtualMemory()
	if err != nil {
		return cpuPercent[0], 0
	}

	return cpuPercent[0], memStats.UsedPercent
}

func getProcessesIO() ([]ProcessIO, error) {
	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var processStats []ProcessIO
	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}

		ioStats, err := p.IOCounters()
		if err != nil {
			continue
		}

		cpuPercent, _ := p.CPUPercent()
		memPercent, _ := p.MemoryPercent()

		openFiles, _ := p.OpenFiles()
		files := make([]string, 0)
		for _, f := range openFiles {
			if f.Path != "" {
				files = append(files, f.Path)
			}
		}

		processStats = append(processStats, ProcessIO{
			PID:        p.Pid,
			Name:       name,
			ReadBytes:  float64(ioStats.ReadBytes),
			WriteBytes: float64(ioStats.WriteBytes),
			OpenFiles:  files,
			CPUPercent: cpuPercent,
			MemPercent: memPercent,
		})
	}

	sort.Slice(processStats, func(i, j int) bool {
		return processStats[i].CPUPercent > processStats[j].CPUPercent
	})

	return processStats, nil
}

func (m model) Init() tea.Cmd {
	return tick
}

func tick() tea.Msg {
	time.Sleep(time.Second)
	return tickMsg{}
}

type tickMsg struct{}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.selected > 0 {
				m.selected--
			}
		case "down":
			if m.selected < len(m.processes)-1 {
				m.selected++
			}
		case "enter":
			m.showFiles = !m.showFiles
		}
	case tickMsg:
		var err error
		m.processes, err = getProcessesIO()
		if err != nil {
			log.Printf("Error getting processes: %v", err)
		}
		return m, tick
	}
	return m, nil
}

func (m model) View() string {
	cpuPercent, memPercent := getSystemStats()
	s := fmt.Sprintf("CPU Usage: %.1f%% | Memory Usage: %.1f%%\n\n", cpuPercent, memPercent)
	s += "PID\tName\tCPU%\tMEM%\tRead/s\tWrite/s\n"
	s += strings.Repeat("-", 80) + "\n"

	for i, p := range m.processes {
		cursor := " "
		if i == m.selected {
			cursor = ">"
			if m.showFiles {
				s += fmt.Sprintf("%s%d\t%s\t%.1f\t%.1f\t%s\t%s\n", cursor, p.PID, p.Name, p.CPUPercent, p.MemPercent, humanizeBytes(p.ReadBytes), humanizeBytes(p.WriteBytes))
				s += "Open files:\n"
				for _, file := range p.OpenFiles {
					s += fmt.Sprintf("\t%s\n", file)
				}
				continue
			}
		}
		s += fmt.Sprintf("%s%d\t%s\t%.1f\t%.1f\t%s\t%s\n", cursor, p.PID, p.Name, p.CPUPercent, p.MemPercent, humanizeBytes(p.ReadBytes), humanizeBytes(p.WriteBytes))
	}

	s += "\nPress up/down to select process, enter to show/hide files, q to quit"
	return s
}

func main() {
	p := tea.NewProgram(model{})
	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}
