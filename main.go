
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

type SortBy int

const (
	SortByCPU SortBy = iota
	SortByRead
	SortByWrite
)

var currentSort SortBy

type ProcessIO struct {
	PID         int32
	Name        string
	ReadBytes   float64
	WriteBytes  float64
	LastRead    float64
	LastWrite   float64
	ReadRate    float64
	WriteRate   float64
	OpenFiles   []string
	CPUPercent  float64
	MemPercent  float32
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

		currentRead := float64(ioStats.ReadBytes)
		currentWrite := float64(ioStats.WriteBytes)
		
		// Find previous stats to calculate rate
		var readRate, writeRate float64
		for _, prev := range processStats {
			if prev.PID == p.Pid {
				readRate = currentRead - prev.LastRead
				writeRate = currentWrite - prev.LastWrite
				break
			}
		}
		
		processStats = append(processStats, ProcessIO{
			PID:         p.Pid,
			Name:        name,
			ReadBytes:   currentRead,
			WriteBytes:  currentWrite,
			LastRead:    currentRead,
			LastWrite:   currentWrite,
			ReadRate:    readRate,
			WriteRate:   writeRate,
			OpenFiles:   files,
			CPUPercent:  cpuPercent,
			MemPercent:  memPercent,
		})
	}

	sort.Slice(processStats, func(i, j int) bool {
		switch currentSort {
		case SortByRead:
			return processStats[i].ReadRate > processStats[j].ReadRate
		case SortByWrite:
			return processStats[i].WriteRate > processStats[j].WriteRate
		default:
			return processStats[i].CPUPercent > processStats[j].CPUPercent
		}
	})

	return processStats, nil
}

func main() {
	if err := ui.Init(); err != nil {
		log.Fatalf("failed to initialize termui: %v", err)
	}
	defer ui.Close()
	
	currentSort = SortByCPU

	table := widgets.NewTable()
	table.TextStyle = ui.NewStyle(ui.ColorWhite)
	table.RowSeparator = true
	table.BorderStyle = ui.NewStyle(ui.ColorGreen)
	table.FillRow = true
	table.Rows = make([][]string, 0)
	table.RowStyles = make(map[int]ui.Style)
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
				humanizeBytes(p.ReadRate),
				humanizeBytes(p.WriteRate),
				func() string {
					if len(p.OpenFiles) == 0 {
						return "-"
					}
					files := p.OpenFiles
					if len(files) > 3 {
						files = files[:3]
					}
					return strings.Join(files, "\n")
				}(),
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
			case "r":
				currentSort = SortByRead
				draw()
			case "w":
				currentSort = SortByWrite
				draw()
			case "c":
				currentSort = SortByCPU
				draw()
			case "<Resize>":
				draw()
			}
		case <-ticker:
			draw()
		}
	}
}
