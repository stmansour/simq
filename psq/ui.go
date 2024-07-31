package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
	"github.com/stmansour/simq/data"
	"github.com/stmansour/simq/util"
	terminal "golang.org/x/term"
)

func printStartupMessage() {
	blue := color.New(color.FgBlue).SprintFunc()
	cyan := color.New(color.FgCyan).SprintFunc()
	//yellow := color.New(color.FgYellow).SprintFunc()

	logo := []string{
		"   ██████╗ ███████╗ ██████╗ ",
		"   ██╔══██╗██╔════╝██╔═══██╗",
		"   ██████╔╝███████╗██║   ██║",
		"   ██╔═══╝ ╚════██║██║▄▄ ██║",
		"   ██║     ███████║╚██████╔╝",
		"   ╚═╝     ╚══════╝ ╚══▀▀═╝ ",
	}

	message := []string{
		"",
	}
	message = append(message, logo...)
	message = append(message,
		"",
		fmt.Sprintf("   Version %s | %s", util.VersionMajorMinor(), util.VersionInfo.BuildID),
		fmt.Sprintf("   Dispatcher: %s", app.DispatcherURL),
		fmt.Sprintf("   Working Dir: %s", getCurrentDirectory()),
		"",
		"   Type 'help' for commands | ↑↓ arrows for command history",
		"",
	)

	printBox(message, blue, cyan)
}

func printBox(lines []string, borderColor, contentColor func(a ...interface{}) string) {
	width := 90
	if w, _, err := terminal.GetSize(int(os.Stdout.Fd())); err == nil {
		// fmt.Printf("Width: %d, w = %d\n", width, w)
		width = w
	}
	width -= 4
	// fmt.Printf("Width: %d\n", width)

	contentWidth := 0
	for _, line := range lines {
		strippedLine := stripAnsi(line)
		lineWidth := runewidth.StringWidth(strippedLine)
		// fmt.Printf("line: %q, strippedLine: %q, runewidth.StringWidth: %d\n", line, strippedLine, lineWidth)
		if lineWidth > contentWidth {
			contentWidth = lineWidth
		}
	}

	if contentWidth < width {
		width = contentWidth + 5
	}
	if width < 80 {
		width = 80
	}
	// fmt.Printf("contentWidth: %d, width: %d\n", contentWidth, width)

	fmt.Println(borderColor("┌" + strings.Repeat("─", width+2) + "┐"))
	for _, line := range lines {
		strippedLine := stripAnsi(line)
		lineWidth := runewidth.StringWidth(strippedLine)
		padding := width - lineWidth
		if runewidth.StringWidth(line) != lineWidth {
			padding++ // Adjust padding for lines with wide characters
		}
		// fmt.Printf("line: %q, strippedLine: %q, lineWidth: %d, padding: %d\n", line, strippedLine, lineWidth, padding)
		if padding < 0 {
			padding = 0
		}
		s := fmt.Sprintf("%s %s%s %s\n",
			borderColor("│"),
			contentColor(line),
			strings.Repeat(" ", padding),
			borderColor("│"))
		fmt.Print(s)
	}
	fmt.Println(borderColor("└" + strings.Repeat("─", width+2) + "┘"))
}

func stripAnsi(str string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(str, "")
}

func getCurrentDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}

func printSimulationStatus(s *data.QueueItem) {
	width := 80
	//--------------------------------------------------------------------------
	// Header
	//--------------------------------------------------------------------------
	printBorder("┏", "━", "┓", width)
	printCenteredText("SIMULATION STATUS", width)
	printBorder("┣", "━", "┫", width)

	//--------------------------------------------------------------------------
	// Main info
	//--------------------------------------------------------------------------
	printTwoColumnRow(fmt.Sprintf("        SID: %d", s.SID), fmt.Sprintf(" Created: %s", s.Created.Format("Jan 02, 2006 03:04pm")), width)
	printTwoColumnRow(fmt.Sprintf("   Username: %s", s.Username), fmt.Sprintf("Modified: %s", s.Modified.Format("Jan 02, 2006 03:04pm")), width)
	printTwoColumnRow(fmt.Sprintf("   Priority: %d", s.Priority), "", width)
	printBorder("┣", "━", "┫", width)

	//--------------------------------------------------------------------------
	// Name, File, and MachineID
	//--------------------------------------------------------------------------
	fmt.Printf("┃        Name: %-64s┃\n", s.Name)
	fmt.Printf("┃ Config File: %-64s┃\n", s.File)
	fmt.Printf("┃   MachineID: %-64s┃\n", s.MachineID)
	printBorder("┣", "━", "┫", width)

	//--------------------------------------------------------------------------
	// Status
	//--------------------------------------------------------------------------
	fmt.Printf("┃      Status:%s┃\n", strings.Repeat(" ", 65))
	printStatusBoxes(s.State, width)
	printProgressArrow(s.State, width)
	printEstimateOrCompleted(s, width)
	printBorder("┣", "━", "┫", width)

	//--------------------------------------------------------------------------
	// Current State
	//--------------------------------------------------------------------------
	fmt.Printf("┃       State: %-64s┃\n", getStateName(s.State))
	printBorder("┣", "━", "┫", width)

	//--------------------------------------------------------------------------
	// Description
	//--------------------------------------------------------------------------
	fmt.Printf("┃ Description:%s┃\n", strings.Repeat(" ", 65))
	description := s.Description
	fmt.Printf("┃ %-77s┃\n", description)

	//--------------------------------------------------------------------------
	// Footer
	//--------------------------------------------------------------------------
	printBorder("┗", "━", "┛", width)
}

func printBorder(left, middle, right string, width int) {
	fmt.Printf("%s%s%s\n", left, strings.Repeat(middle, width-2), right)
}

func printCenteredText(text string, width int) {
	padding := (width - len(text) - 2) / 2
	fmt.Printf("┃%s%s%s┃\n", strings.Repeat(" ", padding), text, strings.Repeat(" ", width-padding-len(text)-2))
}

func printTwoColumnRow(left, right string, _ int) {
	fmt.Printf("┃ %-37s┃ %-38s┃\n", truncateMiddle(left, 36), truncateMiddle(right, 37))
}

func printStatusBoxes(state int, _ int) {
	states := []string{"Queued", "Booked", "Executing", "Finished", "Archived"}
	boxes := make([]string, len(states))
	for i, s := range states {
		if i <= state {
			boxes[i] = fmt.Sprintf("[✓] %-8s", s)
		} else {
			boxes[i] = fmt.Sprintf("[ ] %-8s", s)
		}
	}
	fmt.Printf("┃ %-77s┃\n", strings.Join(boxes, "    "))
}

func printProgressArrow(state int, _ int) {
	arrowLength := (state + 1) * 15
	if arrowLength > 75 {
		arrowLength = 75
	}
	arrow := strings.Repeat("═", arrowLength) + "▶" + strings.Repeat(" ", 75-arrowLength)
	fmt.Printf("┃ %-77s┃\n", arrow)
}

func printEstimateOrCompleted(s *data.QueueItem, _ int) {
	var timeStr string
	if s.State == 2 && s.DtEstimate.Valid {
		timeStr = fmt.Sprintf(" Estimate: %s", s.DtEstimate.Time.Format("Jan 02, 2006 03:04pm"))
	} else if s.State >= 3 && s.DtCompleted.Valid {
		timeStr = fmt.Sprintf("Completed: %s", s.DtCompleted.Time.Format("Jan 02, 2006 03:04pm"))
	}
	if timeStr != "" {
		fmt.Printf("┃%47s%-17s┃\n", "", timeStr)
	} else {
		fmt.Printf("┃%s┃\n", strings.Repeat(" ", 78))
	}
}

func getStateName(state int) string {
	states := []string{"Queued", "Booked", "Executing", "Finished", "Archived", "Error"}
	if state >= 0 && state < len(states) {
		return states[state]
	}
	return "Unknown"
}
