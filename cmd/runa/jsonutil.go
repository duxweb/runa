package main

import (
	"encoding/json"
	"io"
)

func writeJSON(writer io.Writer, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if _, err := writer.Write(body); err != nil {
		return err
	}
	_, err = writer.Write([]byte("\n"))
	return err
}
