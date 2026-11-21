package runner

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/runbrick/reva/internal/model"
)

const (
	maxIterationBytes    = 1 << 20
	maxIterationRows     = 1000
	maxIterationFields   = 128
	maxIterationKeyBytes = 256
	maxIterationValBytes = 64 << 10
	maxIterationRequests = 10000
)

var ErrInvalidDataFile = errors.New("invalid or unreadable iteration data file")

// IterationData owns validated in-memory rows in input order.
type IterationData struct {
	rows [][]model.KV
}

// ReadDataFile reads a bounded regular CSV or JSON file. It returns a fixed
// error so paths, field names and values never become diagnostics.
func ReadDataFile(path string) (IterationData, error) {
	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".csv" && extension != ".json" {
		return IterationData{}, ErrInvalidDataFile
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxIterationBytes {
		return IterationData{}, ErrInvalidDataFile
	}
	file, err := os.Open(path)
	if err != nil {
		return IterationData{}, ErrInvalidDataFile
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxIterationBytes {
		return IterationData{}, ErrInvalidDataFile
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxIterationBytes+1))
	if err != nil || len(raw) > maxIterationBytes || !utf8.Valid(raw) {
		return IterationData{}, ErrInvalidDataFile
	}
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	var rows [][]model.KV
	if extension == ".csv" {
		rows = csvRows(csv.NewReader(bytes.NewReader(raw)))
	} else {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		rows = jsonRows(decoder)
	}
	if len(rows) == 0 {
		return IterationData{}, ErrInvalidDataFile
	}
	return IterationData{rows: rows}, nil
}

func csvRows(reader *csv.Reader) [][]model.KV {
	headers, err := reader.Read()
	if err != nil || len(headers) == 0 || len(headers) > maxIterationFields {
		return nil
	}
	seen := make(map[string]bool, len(headers))
	for _, key := range headers {
		if !validIterationKey(key, seen) {
			return nil
		}
	}
	var rows [][]model.KV
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			return rows
		}
		if err != nil || len(rows) == maxIterationRows {
			return nil
		}
		row := make([]model.KV, len(fields))
		for index, value := range fields {
			if len(value) > maxIterationValBytes {
				return nil
			}
			row[index] = model.KV{Key: headers[index], Value: value}
		}
		rows = append(rows, row)
	}
}

func jsonRows(decoder *json.Decoder) [][]model.KV {
	if token, err := decoder.Token(); err != nil || token != json.Delim('[') {
		return nil
	}
	var rows [][]model.KV
	for decoder.More() {
		if len(rows) == maxIterationRows {
			return nil
		}
		row := jsonRow(decoder)
		if len(row) == 0 {
			return nil
		}
		rows = append(rows, row)
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim(']') {
		return nil
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil
	}
	return rows
}

func jsonRow(decoder *json.Decoder) []model.KV {
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil
	}
	var row []model.KV
	seen := make(map[string]bool)
	for decoder.More() {
		if len(row) == maxIterationFields {
			return nil
		}
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || !validIterationKey(key, seen) {
			return nil
		}
		token, err = decoder.Token()
		if err != nil {
			return nil
		}
		var value string
		switch scalar := token.(type) {
		case string:
			value = scalar
		case json.Number:
			value = scalar.String()
		case bool:
			value = "false"
			if scalar {
				value = "true"
			}
		default:
			return nil
		}
		if len(value) > maxIterationValBytes {
			return nil
		}
		row = append(row, model.KV{Key: key, Value: value})
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return nil
	}
	return row
}

func validIterationKey(key string, seen map[string]bool) bool {
	if key == "" || len(key) > maxIterationKeyBytes || key != strings.TrimSpace(key) ||
		strings.ContainsAny(key, "{}") || strings.ContainsFunc(key, unicode.IsControl) || seen[key] {
		return false
	}
	seen[key] = true
	return true
}
