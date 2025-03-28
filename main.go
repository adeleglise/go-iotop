
package main

import (
	"fmt"
	"log"
	"sort"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/shirou/gopsutil/v3/process"
)

type ProcessIO struct {
	PID       int32
	Name      string
	ReadBytes float64
	WriteBytes float64
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

		processStats = append(processStats, ProcessIO{
			PID:        p.Pid,
			Name:       name,
			ReadBytes:  float64(ioStats.ReadBytes),
			WriteBytes: float64(ioStats.WriteBytes),
		})
	}

	sort.Slice(processStats, func(i, j int) bool {
		return processStats[i].ReadBytes+processStats[i].WriteBytes > 
		       processStats[j].ReadBytes+processStats[j].WriteBytes
	})

	return processStats, nil
}

func main() {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	table := widgets.NewTable()
	table.Rows = [][]string{
		{"PID", "Name", "Read (B/s)", "Write (B/s)"},
	}
	table.TextStyle = ui.NewStyle(ui.ColorWhite)
	table.BorderStyle = ui.NewStyle(ui.ColorGreen)
	table.RowSeparator = true
	table.FillRow = true

	draw := func() {
		w, h := ui.TerminalDimensions()
		table.SetRect(0, 0, w, h)

		processes, err := getProcessesIO()
		if err != nil {
			log.Printf("Error getting processes: %v", err)
			return
		}

		rows := [][]string{{"PID", "Name", "Read (B/s)", "Write (B/s)"}}
		maxProcesses := len(processes)
		if maxProcesses > 20 {
			maxProcesses = 20
		}
		for _, p := range processes[:maxProcesses] { // Show available processes up to 20
			rows = append(rows, []string{
				fmt.Sprintf("%d", p.PID),
				p.Name,
				fmt.Sprintf("%.2f", p.ReadBytes),
				fmt.Sprintf("%.2f", p.WriteBytes),
			})
		}
		table.Rows = rows
		ui.Render(table)
	}

	draw()

	uiEvents := ui.PollEvents()
	ticker := time.NewTicker(time.Second).C

	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>":
				return
			case "<Resize>":
				draw()
			}
		case <-ticker:
			draw()
		}
	}
}
