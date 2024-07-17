package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-runewidth"
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
		fmt.Sprintf("   Version 1.0 | %s", time.Now().Format("2006-01-02 15:04:05")),
		fmt.Sprintf("   Dispatcher: %s", "http://216.16.195.147:8250"),
		fmt.Sprintf("   Working Dir: %s", getCurrentDirectory()),
		"",
		"   Type 'help' for commands | ↑↓ arrows for command history",
		"",
	)

	printBox(message, blue, cyan)
}

func printBox(lines []string, borderColor, contentColor func(a ...interface{}) string) {
	width := 80

	if w, _, err := terminal.GetSize(int(os.Stdout.Fd())); err == nil {
		width = w
	}
	width -= 4

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
		width = contentWidth
	}

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

// func printListOutput() {
// 	cyan := color.New(color.FgCyan).SprintFunc()
// 	yellow := color.New(color.FgYellow).SprintFunc()

// 	fmt.Println("\npsq> list")

// 	headers := []string{"SID", "PRI", "ST", "Username", "File", "Estimate", "MachineID", "Name"}
// 	data := [][]string{
// 		{"3", "5", "Ex", "steve", "long.json5", "Jul 16, 2024 07:56pm", "7cf2...7e3587c5faec", "Test-…"},
// 		{"6", "5", "Ex", "steve", "long.json5", "Jul 16, 2024 07:58pm", "7cf2...7e3587c5faec", "Test-…"},
// 		{"9", "5", "Ex", "steve", "long.json5", "Jul 16, 2024 08:07pm", "7cf2...7e3587c5faec", "Test-…"},
// 	}

// 	printTable("ACTIVE SIMULATIONS", headers, data, cyan, yellow)

// 	fmt.Println("\npsq>")
// }

// func printTable(title string, headers []string, data [][]string, borderColor, contentColor func(a ...interface{}) string) {
// 	// Calculate column widths
// 	widths := make([]int, len(headers))
// 	for i, header := range headers {
// 		widths[i] = len(header)
// 		for _, row := range data {
// 			if len(row[i]) > widths[i] {
// 				widths[i] = len(row[i])
// 			}
// 		}
// 	}

// 	// Print title
// 	totalWidth := sum(widths) + len(widths)*3 + 1
// 	fmt.Printf("%s %s %s\n",
// 		borderColor("╭"+strings.Repeat("─", (totalWidth-len(title))/2-2)),
// 		contentColor(title),
// 		borderColor(strings.Repeat("─", (totalWidth-len(title))/2-1)+"╮"))

// 	// Print headers
// 	printRow(headers, widths, borderColor, contentColor)

// 	// Print separator
// 	fmt.Print(borderColor("├"))
// 	for i, w := range widths {
// 		fmt.Print(borderColor(strings.Repeat("─", w+2)))
// 		if i < len(widths)-1 {
// 			fmt.Print(borderColor("┼"))
// 		}
// 	}
// 	fmt.Println(borderColor("┤"))

// 	// Print data
// 	for _, row := range data {
// 		printRow(row, widths, borderColor, contentColor)
// 	}

// 	// Print bottom border
// 	fmt.Println(borderColor("╰" + strings.Repeat("─", totalWidth-2) + "╯"))
// }

// func printRow(row []string, widths []int, borderColor, contentColor func(a ...interface{}) string) {
// 	fmt.Print(borderColor("│"))
// 	for i, cell := range row {
// 		fmt.Printf("%s %*s %s",
// 			contentColor(""),
// 			-widths[i], cell,
// 			borderColor("│"))
// 	}
// 	fmt.Println()
// }

// func sum(nums []int) int {
// 	total := 0
// 	for _, num := range nums {
// 		total += num
// 	}
// 	return total
// }
