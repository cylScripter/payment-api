package initials

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

var consoleJSONKeys = regexp.MustCompile(`("(?:\\.|[^"\\])*")(\s*:)`)

type consoleWriter struct {
	ip    string
	color bool
}

func (w consoleWriter) Write(p []byte) (int, error) {
	var record map[string]any
	decoder := json.NewDecoder(bytes.NewReader(p))
	decoder.UseNumber()
	if err := decoder.Decode(&record); err != nil {
		return os.Stderr.Write(p)
	}

	timestamp, _ := record["time"].(string)
	level, _ := record["level"].(string)
	message, _ := record["msg"].(string)
	method, _ := record["method"].(string)
	path, _ := record["path"].(string)
	for _, key := range []string{"time", "level", "msg", "method", "path"} {
		delete(record, key)
	}
	levelColor := "90"
	switch level {
	case "info":
		levelColor = "32"
	case "warn", "warning":
		levelColor = "33"
	case "error", "fatal", "panic":
		levelColor = "31"
	}
	parts := []string{fmt.Sprintf("%s %s %s",
		w.style(timestamp, "90"),
		w.style("["+strings.ToUpper(level)+"]", levelColor),
		w.style("["+w.ip+"]", "1;34"),
	)}
	if method != "" || path != "" {
		parts = append(parts, strings.TrimSpace(w.style(method, "36")+" "+path))
	}
	parts = append(parts, message)
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		var text string
		var isJSON bool
		if value, ok := record[key].(string); ok {
			if value == "" {
				continue
			}
			text = value
			if key == "body" {
				var compact bytes.Buffer
				if json.Compact(&compact, []byte(value)) == nil {
					text = compact.String()
					isJSON = true
				}
			}
		} else {
			encoded, _ := json.Marshal(record[key])
			text = string(encoded)
			isJSON = true
		}
		if w.color && isJSON {
			text = consoleJSONKeys.ReplaceAllString(text, "\033[1;34m$1\033[0m$2")
		}
		if key == "file" || key == "func" {
			text = w.style(text, "90")
		}
		parts = append(parts, w.style(key, "1;34")+"="+text)
	}
	line := strings.Join(parts, w.style(" | ", "90"))
	line = strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(line)
	if _, err := fmt.Fprintln(os.Stderr, line); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w consoleWriter) style(text, code string) string {
	if !w.color {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
}
