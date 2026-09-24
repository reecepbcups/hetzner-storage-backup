// Package cosmetics ports src/cosmetics.py: "&"-code colored terminal output.
package cosmetics

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var colorDict = map[string]string{
	"&1": "\u001b[38;5;4m",
	"&2": "\u001b[38;5;2m",
	"&3": "\u001b[38;5;6m",
	"&4": "\u001b[38;5;1m",
	"&5": "\u001b[38;5;5m",
	"&6": "\u001b[38;5;3m",
	"&7": "\u001b[38;5;7m",
	"&8": "\u001b[38;5;8m",
	"&0": "\u001b[38;5;0m",
	"&a": "\u001b[38;5;10m",
	"&b": "\u001b[38;5;14m",
	"&c": "\u001b[38;5;9m",
	"&d": "\u001b[38;5;13m",
	"&e": "\u001b[38;5;11m",
	"&f": "\u001b[38;5;15m",
	"&r": "\u001b[0m",
}

// Color translates "&"-color codes in text into terminal escape codes.
func Color(text string) string {
	text = strings.ReplaceAll(text, "§", "&")
	var b strings.Builder
	i := 0
	for i < len(text) {
		if i+2 <= len(text) {
			if code, ok := colorDict[text[i:i+2]]; ok {
				b.WriteString(code)
				i += 2
				continue
			}
		}
		b.WriteByte(text[i])
		i++
	}
	b.WriteString(colorDict["&r"])
	return b.String()
}

// CPrint prints text with color codes translated.
func CPrint(text string) {
	fmt.Println(Color(text))
}

// CInput prompts with a colored message and returns the trimmed line.
// Ctrl+C (EOF) exits the process, matching the Python KeyboardInterrupt handling.
func CInput(text string) string {
	fmt.Print(Color(text))
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		CPrint("\n&cKeyboard Interrupt, Exiting...\n")
		os.Exit(0)
	}
	return strings.TrimRight(line, "\n")
}

var splitColorsRe = regexp.MustCompile(`(&[a-zA-Z0-9])`)

// SplitColors splits a string on its color codes, keeping the codes.
// "&at&bt&ct" -> ["", "&a", "t", "&b", "t", "&c", "t"]
func SplitColors(s string) []string {
	matches := splitColorsRe.FindAllStringIndex(s, -1)
	var result []string
	last := 0
	for _, m := range matches {
		result = append(result, s[last:m[0]])
		result = append(result, s[m[0]:m[1]])
		last = m[1]
	}
	result = append(result, s[last:])
	return result
}
