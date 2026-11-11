// Package format contains pure presentation helpers for HTTP responses.
package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime"
	"strings"
	"time"
)

// PrettyJSON indents JSON with two spaces without decoding it into a map.
// Object order, duplicate keys, number spelling and string escapes are preserved.
// Leading and trailing whitespace is removed for a stable display.
func PrettyJSON(body []byte) (string, error) {
	var out bytes.Buffer
	if err := json.Indent(&out, bytes.TrimSpace(body), "", "  "); err != nil {
		return "", err
	}
	return out.String(), nil
}

// IsJSON recognizes a JSON media type (including structured +json types), or a
// valid JSON body even when the server omitted or misreported Content-Type.
func IsJSON(contentType string, body []byte) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		mediaType = strings.ToLower(mediaType)
		if mediaType == "application/json" || mediaType == "text/json" || strings.HasSuffix(mediaType, "+json") {
			return true
		}
	}
	return json.Valid(body)
}

// ByteSize displays bytes using binary KB and MB units.
func ByteSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.2f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%.2f MB", float64(n)/(1024*1024))
	}
}

// Duration displays whole milliseconds below a second, otherwise seconds with
// two decimal places.
func Duration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}
