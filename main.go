
package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	ui "github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type ProcessIO struct {
	PID        int32
	Name       string
	ReadBytes  float64
	WriteBytes float64
	OpenFiles  []string
	CPUPercent float64
	MemPercent float32
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

func getSystemStats() (*widgets.Gauge, *widgets.Gauge, error) {
	cpuGauge := widgets.NewGauge()
	cpuGauge.Title = "CPU Usage"
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		cpuGauge.Percent = int(cpuPercent[0])
	}
	
	memGauge := widgets.NewGauge()
	memGauge.Title = "Memory Usage"
	memStats, err := mem.VirtualMemory()
	if err == nil {
		memGauge.Percent = int(memStats.UsedPercent)
	}

	return cpuGauge, memGauge, err
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

func main() {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()

	table := widgets.NewTable()
	table.TextStyle = ui.NewStyle(ui.ColorWhite)
	table.RowSeparator = true
	table.BorderStyle = ui.NewStyle(ui.ColorGreen)
	table.FillRow = true
	table.RowStyles[0] = ui.NewStyle(ui.ColorYellow, ui.ColorClear, ui.ModifierBold)

	draw := func() {
		w, h := ui.TerminalDimensions()
		
		cpuGauge, memGauge, _ := getSystemStats()
		cpuGauge.SetRect(0, 0, w/2, 3)
		memGauge.SetRect(w/2, 0, w, 3)
		
		table.SetRect(0, 3, w, h)
		
		processes, err := getProcessesIO()
		if err != nil {
			log.Printf("Error getting processes: %v", err)
			return
		}

		rows := [][]string{{"PID", "Name", "CPU%", "MEM%", "Read/s", "Write/s", "Open Files"}}
		maxProcesses := len(processes)
		if maxProcesses > 20 {
			maxProcesses = 20
		}

		table.ColumnWidths = []int{8, 30, 8, 8, 12, 12, 0} // Adjust column widths, last column takes remaining space
		
		for _, p := range processes[:maxProcesses] {
			rows = append(rows, []string{
				fmt.Sprintf("%d", p.PID),
				p.Name,
				fmt.Sprintf("%.1f", p.CPUPercent),
				fmt.Sprintf("%.1f", p.MemPercent),
				humanizeBytes(p.ReadBytes),
				humanizeBytes(p.WriteBytes),
				strings.Join(p.OpenFiles[:min(len(p.OpenFiles), 3)], ", "),
			})
		}
		table.Rows = rows

		ui.Render(cpuGauge, memGauge, table)
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
