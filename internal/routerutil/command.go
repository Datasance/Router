package routerutil

import (
	"bytes"
)

const (
	maxLineLength = 80
	indent        = "   "
	newLinePrefix = " \\\n" + indent
)

func PrettyPrintCommand(command string, args []string) string {
	var lineLength int
	buf := new(bytes.Buffer)
	_, _ = buf.WriteString(command)
	lineLength = len(command)
	for i, arg := range args {
		_, _ = buf.WriteString(" ")
		_, _ = buf.WriteString(arg)
		lineLength += len(arg) + 1
		if lineLength > maxLineLength && i < len(args)-1 {
			_, _ = buf.WriteString(newLinePrefix)
			lineLength = len(indent)
		}
	}
	_, _ = buf.WriteString("\n")
	return buf.String()
}
