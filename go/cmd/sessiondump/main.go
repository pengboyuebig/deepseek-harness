// Command sessiondump prints the derived messages of JSONL session fixtures as
// one JSON line per file, the Go side of the session parity harness.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"git.glkj.example.com/glkj/harness/go/internal/core/session"
)

func main() {
	for _, path := range os.Args[1:] {
		result := map[string]any{"file": path}
		log, err := session.ReadFile(path)
		if err != nil {
			result["error"] = err.Error()
		} else if messages, deriveErr := log.DeriveMessages(); deriveErr != nil {
			result["error"] = deriveErr.Error()
		} else {
			raw := make([]json.RawMessage, 0, len(messages))
			for _, message := range messages {
				encoded, marshalErr := json.Marshal(message)
				if marshalErr != nil {
					result["error"] = marshalErr.Error()
					raw = nil
					break
				}
				raw = append(raw, encoded)
			}
			if raw != nil {
				result["messages"] = raw
			}
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sessiondump: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(encoded))
	}
}
