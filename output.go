package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	green  = color.New(color.FgGreen, color.Bold).SprintfFunc()
	red    = color.New(color.FgRed, color.Bold).SprintfFunc()
	yellow = color.New(color.FgYellow, color.Bold).SprintfFunc()
	cyan   = color.New(color.FgCyan).SprintfFunc()
	white  = color.New(color.FgWhite, color.Bold).SprintfFunc()
)

const userAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0"

func logInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] %s\n", cyan("+"), msg)
}

func logSuccess(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] %s\n", green("✓"), msg)
}

func logError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] %s\n", red("✗"), msg)
}

func logWarn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] %s\n", yellow("!"), msg)
}

func logDebug(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] %s\n", white("-"), msg)
}

func logProgress(attempted int, total int, rate float64, elapsed float64, found int) {
	progressLine := fmt.Sprintf("\r[%s] Intentos: %d/%d | Tasa: %.0f/s | Transcurrido: %.0fs | Encontrados: %d",
		cyan("→"), attempted, total, rate, elapsed, found)
	fmt.Fprint(os.Stderr, progressLine)
}

func logProgressDone() {
	fmt.Fprintln(os.Stderr)
}

func WriteOutput(path string, found []FoundCredential) error {
	if len(found) == 0 || path == "" {
		return nil
	}

	var sb strings.Builder
	for _, f := range found {
		sb.WriteString(fmt.Sprintf("%s:%s\n", f.Username, f.Password))
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("error al escribir el archivo de salida: %w", err)
	}
	return nil
}

func PrintBanner() {
	banner := `
                _                 _                _
               | |               | |              | |
__  ___ __ ___ | |_ __ _ __   ___| |__  _ __ _   _| |_ ___
\ \/ / '_ ' _ \| | '__| '_ \ / __| '_ \| '__| | | | __/ _ \
 >  <| | | | | | | |  | |_) | (__| |_) | |  | |_| | ||  __/
_/\/_\_| |_| |_|_|_|  | .__/ \___|_.__/|_|   \__,_|\__\___|
                      | |
                      |_|
`
	fmt.Fprintf(os.Stderr, "%s\n", color.CyanString(banner))
	fmt.Fprintf(os.Stderr, "%s\n", color.CyanString("WordPress XML-RPC Brute Forcer (system.multicall)"))
	fmt.Fprintln(os.Stderr)
}
