package toolbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

type Tool int

const (
	Base64 Tool = iota
	URL
	JWT
	JSONEscape
	Unicode
	Hash
	HMAC
	AES
	JSON
	XML
	Hex
	Image
	Color
	Timestamp
	UUID
	Regex
	Count
)

type Algorithm int

const (
	SHA256 Algorithm = iota
	SHA512
	SHA1
	MD5
)

func (algorithm Algorithm) String() string {
	return [...]string{"SHA-256", "SHA-512", "SHA-1", "MD5"}[algorithm]
}

const (
	MaxInput     = 64 << 10
	MaxImageSize = 2 << 20
	maxOutput    = 512 << 10
)

var (
	ErrInput   = errors.New("invalid input")
	ErrLimit   = errors.New("input or result exceeds limits")
	ErrKey     = errors.New("invalid key")
	ErrDecrypt = errors.New("ciphertext authentication failed")
	ErrBinary  = errors.New("result is not UTF-8 text")
	ErrNoMatch = errors.New("no matches")
	ErrImage   = errors.New("invalid or unsupported image")
)

type Job struct {
	Tool      Tool
	Input     string
	Aux       string
	Reverse   bool
	Alternate bool
	Algorithm Algorithm
}

type Result struct {
	Text   string
	Image  []byte
	Color  [4]byte
	Swatch bool
}

func Run(job Job) (Result, error) {
	limit := MaxInput
	if job.Tool == Image {
		limit = MaxImageSize
	}
	if len(job.Input) > limit || len(job.Aux) > MaxInput {
		return Result{}, ErrLimit
	}
	if !utf8.ValidString(job.Input) || !utf8.ValidString(job.Aux) {
		return Result{}, ErrInput
	}
	var value string
	var err error
	switch job.Tool {
	case Base64, Hex:
		value, err = byteCodec(job)
	case URL:
		value, err = urlCodec(job)
	case JWT:
		value, err = inspectJWT(job.Input)
	case JSONEscape, Unicode:
		value, err = textCodec(job)
	case Hash, HMAC:
		value, err = digest(job)
	case AES:
		value, err = aesGCM(job)
	case JSON:
		value, err = jsonView(job)
	case XML:
		value, err = xmlView(job.Input)
	case Image:
		data, err := imageBytes(job.Input)
		return Result{Image: data}, err
	case Color:
		return colorValue(job.Input)
	case Timestamp:
		value, err = timestamp(job)
	case UUID:
		value = uuid()
	case Regex:
		value, err = regexFind(job.Aux, job.Input)
	default:
		err = ErrInput
	}
	if err != nil {
		return Result{}, err
	}
	if len(value) > maxOutput {
		return Result{}, ErrLimit
	}
	return Result{Text: value}, nil
}

func byteCodec(job Job) (string, error) {
	if job.Tool == Hex {
		if !job.Reverse {
			return hex.EncodeToString([]byte(job.Input)), nil
		}
		return decodedText(hex.DecodeString(strings.Join(strings.Fields(job.Input), "")))
	}
	encoding := base64.StdEncoding
	if job.Alternate {
		encoding = base64.RawURLEncoding
	}
	if !job.Reverse {
		return encoding.EncodeToString([]byte(job.Input)), nil
	}
	return decodedText(decodeBase64(job.Input, job.Alternate))
}

func decodeBase64(value string, urlSafe bool) ([]byte, error) {
	value = strings.Join(strings.Fields(value), "")
	encoding := base64.StdEncoding
	if urlSafe {
		encoding = base64.URLEncoding
	}
	if !strings.HasSuffix(value, "=") {
		encoding = encoding.WithPadding(base64.NoPadding)
	}
	return encoding.Strict().DecodeString(value)
}

func decodedText(data []byte, err error) (string, error) {
	if err != nil {
		return "", ErrInput
	}
	if !utf8.Valid(data) {
		return "", ErrBinary
	}
	return string(data), nil
}

func urlCodec(job Job) (string, error) {
	if !job.Reverse {
		value := url.QueryEscape(job.Input)
		if !job.Alternate {
			value = strings.ReplaceAll(value, "+", "%20")
		}
		return value, nil
	}
	decode := url.PathUnescape
	if job.Alternate {
		decode = url.QueryUnescape
	}
	value, err := decode(job.Input)
	if err != nil {
		return "", ErrInput
	}
	if !utf8.ValidString(value) {
		return "", ErrBinary
	}
	return value, nil
}

func textCodec(job Job) (string, error) {
	if job.Reverse {
		if job.Tool == Unicode {
			return decodeUnicode(job.Input)
		}
		value := job.Input
		if !strings.HasPrefix(strings.TrimSpace(value), `"`) {
			value = `"` + value + `"`
		}
		var decoded string
		if json.Unmarshal([]byte(value), &decoded) != nil {
			return "", ErrInput
		}
		return decoded, nil
	}
	if job.Tool == JSONEscape {
		encoded, err := json.Marshal(job.Input)
		return string(encoded), err
	}
	var out strings.Builder
	for _, char := range job.Input {
		if char <= 0xffff {
			fmt.Fprintf(&out, `\u%04x`, char)
		} else {
			high, low := utf16.EncodeRune(char)
			fmt.Fprintf(&out, `\u%04x\u%04x`, high, low)
		}
	}
	return out.String(), nil
}

func decodeUnicode(input string) (string, error) {
	var out strings.Builder
	for input != "" {
		if !strings.HasPrefix(input, `\u`) {
			char, width := utf8.DecodeRuneInString(input)
			out.WriteRune(char)
			input = input[width:]
			continue
		}
		read := func(value string) (rune, bool) {
			if len(value) < 6 || !strings.HasPrefix(value, `\u`) {
				return 0, false
			}
			var pair [2]byte
			if _, err := hex.Decode(pair[:], []byte(value[2:6])); err != nil {
				return 0, false
			}
			return rune(pair[0])<<8 | rune(pair[1]), true
		}
		char, ok := read(input)
		if !ok {
			return "", ErrInput
		}
		input = input[6:]
		if utf16.IsSurrogate(char) {
			low, ok := read(input)
			if !ok {
				return "", ErrInput
			}
			char = utf16.DecodeRune(char, low)
			if char == utf8.RuneError {
				return "", ErrInput
			}
			input = input[6:]
		}
		out.WriteRune(char)
	}
	return out.String(), nil
}

func digest(job Job) (string, error) {
	var newHash func() hash.Hash
	switch job.Algorithm {
	case SHA256:
		newHash = sha256.New
	case SHA512:
		newHash = sha512.New
	case SHA1:
		newHash = sha1.New
	case MD5:
		newHash = md5.New
	default:
		return "", ErrInput
	}
	var digest hash.Hash
	if job.Tool == HMAC {
		if job.Aux == "" || job.Algorithm != SHA256 && job.Algorithm != SHA512 {
			return "", ErrKey
		}
		digest = hmac.New(newHash, []byte(job.Aux))
	} else {
		digest = newHash()
	}
	digest.Write([]byte(job.Input))
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func aesGCM(job Job) (string, error) {
	key, err := hex.DecodeString(strings.TrimSpace(job.Aux))
	if err != nil {
		return "", ErrKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrKey
	}
	gcm, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return "", ErrKey
	}
	if job.Reverse {
		ciphertext, err := decodeBase64(job.Input, false)
		if err != nil {
			return "", ErrInput
		}
		plaintext, err := gcm.Open(nil, nil, ciphertext, nil)
		if err != nil {
			return "", ErrDecrypt
		}
		return decodedText(plaintext, nil)
	}
	if base64.StdEncoding.EncodedLen(len(job.Input)+gcm.Overhead()) > MaxInput {
		return "", ErrLimit
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nil, nil, []byte(job.Input), nil)), nil
}

func timestamp(job Job) (string, error) {
	value := strings.TrimSpace(job.Input)
	var moment time.Time
	if value == "" {
		moment = time.Now()
	} else if number, err := strconv.ParseInt(value, 10, 64); err == nil {
		if job.Alternate {
			moment = time.UnixMilli(number)
		} else {
			if number < -62167219200 || number > 253402300799 {
				return "", ErrInput
			}
			moment = time.Unix(number, 0)
		}
	} else {
		moment, err = time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return "", ErrInput
		}
	}
	moment = moment.UTC()
	if moment.Year() < 0 || moment.Year() > 9999 {
		return "", ErrInput
	}
	return fmt.Sprintf("%s\n%d s\n%d ms", moment.Format(time.RFC3339Nano), moment.Unix(), moment.UnixMilli()), nil
}

func uuid() string {
	var value [16]byte
	rand.Read(value[:])
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[:4], value[4:6], value[6:8], value[8:10], value[10:])
}

func regexFind(pattern, input string) (string, error) {
	const maxMatches = 100
	if len(pattern) > 1024 {
		return "", ErrLimit
	}
	if pattern == "" {
		return "", ErrInput
	}
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return "", ErrInput
	}
	indices := compiled.FindAllStringSubmatchIndex(input, maxMatches+1)
	if len(indices) == 0 {
		return "", ErrNoMatch
	}
	if len(indices) > maxMatches {
		return "", ErrLimit
	}
	var out strings.Builder
	for index, match := range indices {
		fmt.Fprintf(&out, "%d. [%d:%d] %s\n", index+1, match[0], match[1], strconv.Quote(input[match[0]:match[1]]))
		for group := 1; group < len(match)/2; group++ {
			start, end := match[group*2], match[group*2+1]
			if start >= 0 {
				fmt.Fprintf(&out, "   $%d [%d:%d] %s\n", group, start, end, strconv.Quote(input[start:end]))
			}
			if out.Len() > maxOutput {
				return "", ErrLimit
			}
		}
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}
