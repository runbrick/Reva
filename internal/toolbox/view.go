package toolbox

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/runbrick/reva/internal/format"
)

func inspectJWT(input string) (string, error) {
	header, rest, ok := strings.Cut(strings.TrimSpace(input), ".")
	payload, signature, hasSignature := strings.Cut(rest, ".")
	if !ok || !hasSignature {
		return "", ErrInput
	}
	decodedHeader, headerErr := jwtObject(header)
	decodedPayload, payloadErr := jwtObject(payload)
	if headerErr == ErrLimit || payloadErr == ErrLimit {
		return "", ErrLimit
	}
	if _, err := base64.RawURLEncoding.Strict().DecodeString(signature); err != nil || headerErr != nil || payloadErr != nil || strings.ContainsAny(signature, "\r\n") {
		return "", ErrInput
	}
	value, err := json.MarshalIndent(struct {
		Header    json.RawMessage `json:"header"`
		Payload   json.RawMessage `json:"payload"`
		Signature string          `json:"signature"`
	}{decodedHeader, decodedPayload, signature}, "", "  ")
	return string(value), err
}

func jwtObject(segment string) (json.RawMessage, error) {
	data, err := base64.RawURLEncoding.Strict().DecodeString(segment)
	data = bytes.TrimSpace(data)
	if err != nil || strings.ContainsAny(segment, "\r\n") || len(data) == 0 || data[0] != '{' {
		return nil, ErrInput
	}
	return data, checkJSON(data)
}

func checkJSON(data []byte) error {
	if !utf8.Valid(data) || !json.Valid(data) {
		return ErrInput
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	depth := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return ErrInput
		}
		switch token {
		case json.Delim('{'), json.Delim('['):
			depth++
			if depth > 64 {
				return ErrLimit
			}
		case json.Delim('}'), json.Delim(']'):
			depth--
		}
	}
}

func jsonView(job Job) (string, error) {
	data := []byte(job.Input)
	if err := checkJSON(data); err != nil {
		return "", err
	}
	if job.Alternate {
		var out bytes.Buffer
		if err := json.Compact(&out, data); err != nil {
			return "", ErrInput
		}
		return out.String(), nil
	}
	return format.PrettyJSON(data)
}

func xmlView(input string) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(input))
	depth, roots := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF && depth == 0 && roots == 1 {
			return input, nil
		}
		if err != nil {
			return "", ErrInput
		}
		switch value := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots > 1 {
					return "", ErrInput
				}
			}
			depth++
			if depth > 64 {
				return "", ErrLimit
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && len(bytes.Trim(value, " \r\n\t")) > 0 {
				return "", ErrInput
			}
		case xml.Directive:
			return "", ErrInput
		}
	}
}

func imageBytes(input string) ([]byte, error) {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "data:") {
		prefix, body, ok := strings.Cut(input, ",")
		if !ok || !strings.HasPrefix(prefix, "data:image/") || !strings.HasSuffix(prefix, ";base64") {
			return nil, ErrImage
		}
		input = body
	}
	data, err := decodeBase64(input, false)
	if err != nil || len(data) == 0 {
		return nil, ErrImage
	}
	return data, nil
}

func colorValue(input string) (Result, error) {
	value := strings.TrimPrefix(strings.TrimSpace(input), "#")
	switch len(value) {
	case 3, 4:
		var expanded strings.Builder
		for _, char := range value {
			expanded.WriteRune(char)
			expanded.WriteRune(char)
		}
		value = expanded.String()
	case 6, 8:
	default:
		return Result{}, ErrInput
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != 3 && len(decoded) != 4 {
		return Result{}, ErrInput
	}
	channels := [4]byte{decoded[0], decoded[1], decoded[2], 255}
	if len(decoded) == 4 {
		channels[3] = decoded[3]
	}
	return Result{
		Text:   fmt.Sprintf("#%X\nrgba(%d, %d, %d, %.4g)", decoded, channels[0], channels[1], channels[2], float64(channels[3])/255),
		Color:  channels,
		Swatch: true,
	}, nil
}
