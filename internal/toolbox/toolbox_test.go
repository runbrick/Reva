package toolbox

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
)

func TestToolboxCodecRoundTrips(t *testing.T) {
	const input = "你好😃\n + / \" &"
	for _, test := range []struct {
		name      string
		tool      Tool
		alternate bool
	}{
		{"base64", Base64, false}, {"base64url", Base64, true}, {"component", URL, false},
		{"form", URL, true}, {"json string", JSONEscape, false}, {"unicode", Unicode, false}, {"hex", Hex, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := Job{Tool: test.tool, Input: input, Alternate: test.alternate}
			encoded, err := Run(job)
			if err != nil || encoded.Text == input || encoded.Text == "" {
				t.Fatalf("encode returned %q, %v", encoded.Text, err)
			}
			job.Input, job.Reverse = encoded.Text, true
			decoded, err := Run(job)
			if err != nil || decoded.Text != input {
				t.Fatalf("decode returned %q, %v; want %q", decoded.Text, err, input)
			}
		})
	}
	check(t, Job{Tool: URL, Input: "a b+c"}, "a%20b%2Bc")
	check(t, Job{Tool: URL, Input: "a+b%2Bc", Reverse: true, Alternate: true}, "a b+c")
	check(t, Job{Tool: Unicode, Input: `字\ud83d\ude03`, Reverse: true}, "字😃")
}

func TestToolboxDigestsAndAuthenticatedAES(t *testing.T) {
	check(t, Job{Tool: Hash, Input: "abc"}, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
	check(t, Job{Tool: HMAC, Aux: "key", Input: "The quick brown fox jumps over the lazy dog"}, "f7bc83f430538424b13298e6aa6fb143ef4d59a14946175997479dbc2d1a3cd8")
	job := Job{Tool: AES, Aux: "00112233445566778899aabbccddeeff", Input: "消息"}
	first, err := Run(job)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(job)
	if err != nil || first.Text == second.Text {
		t.Fatalf("AES did not use fresh nonces: %v", err)
	}
	job.Input, job.Reverse = first.Text, true
	check(t, job, "消息")
	data, err := base64.StdEncoding.DecodeString(first.Text)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 1
	job.Input = base64.StdEncoding.EncodeToString(data)
	if result, err := Run(job); err != ErrDecrypt || result.Text != "" {
		t.Fatalf("tampered ciphertext returned %q, %v", result.Text, err)
	}
}

func TestToolboxViewersAndGenerators(t *testing.T) {
	check(t, Job{Tool: JSON, Input: `{"z":1e3,"z":2}`}, "{\n  \"z\": 1e3,\n  \"z\": 2\n}")
	check(t, Job{Tool: JSON, Input: `[ 1, "x y" ]`, Alternate: true}, `[1,"x y"]`)
	check(t, Job{Tool: XML, Input: `<?xml version="1.0"?><root a="b">hello &amp; <child/></root>`}, `<?xml version="1.0"?><root a="b">hello &amp; <child/></root>`)
	check(t, Job{Tool: JWT, Input: "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2ln"}, "{\n  \"header\": {\n    \"alg\": \"HS256\"\n  },\n  \"payload\": {\n    \"sub\": \"1\"\n  },\n  \"signature\": \"c2ln\"\n}")
	check(t, Job{Tool: Timestamp, Input: "1700000000123", Alternate: true}, "2023-11-14T22:13:20.123Z\n1700000000 s\n1700000000123 ms")
	check(t, Job{Tool: Timestamp, Input: "1970-01-01T08:00:00+08:00"}, "1970-01-01T00:00:00Z\n0 s\n0 ms")
	check(t, Job{Tool: Regex, Aux: `(例)(\d+)`, Input: "中例23"}, "1. [3:8] \"例23\"\n   $1 [3:6] \"例\"\n   $2 [6:8] \"23\"")
	color, err := Run(Job{Tool: Color, Input: "#f608"})
	if err != nil || !color.Swatch || color.Color != [4]byte{255, 102, 0, 136} || !strings.HasPrefix(color.Text, "#FF660088\n") {
		t.Fatalf("color = %+v, %v", color, err)
	}
	first, err := Run(Job{Tool: UUID})
	if err != nil || !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(first.Text) {
		t.Fatalf("UUID = %q, %v", first.Text, err)
	}
}

func TestToolboxRejectsInvalidOrOversizedInputs(t *testing.T) {
	for _, test := range []struct {
		job  Job
		want error
	}{
		{Job{Tool: Base64, Input: "!!", Reverse: true}, ErrInput},
		{Job{Tool: Hex, Input: "ff", Reverse: true}, ErrBinary},
		{Job{Tool: Unicode, Input: `\ud800`, Reverse: true}, ErrInput},
		{Job{Tool: AES, Input: "secret", Aux: "123"}, ErrKey},
		{Job{Tool: JWT, Input: "eyJ9.bnVsbA.sig.extra"}, ErrInput},
		{Job{Tool: JSON, Input: strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65)}, ErrLimit},
		{Job{Tool: XML, Input: `<!DOCTYPE doc SYSTEM "file:///secret"><doc/>`}, ErrInput},
		{Job{Tool: XML, Input: `<a/><b/>`}, ErrInput},
		{Job{Tool: Image, Input: "https://example.test/image.png"}, ErrImage},
		{Job{Tool: Timestamp, Input: "9223372036854775807"}, ErrInput},
		{Job{Tool: Hash, Input: strings.Repeat("a", MaxInput+1)}, ErrLimit},
		{Job{Tool: Regex, Aux: "[", Input: "test"}, ErrInput},
		{Job{Tool: Regex, Aux: "x", Input: "test"}, ErrNoMatch},
		{Job{Tool: Regex, Aux: ".", Input: strings.Repeat("a", 101)}, ErrLimit},
	} {
		if result, err := Run(test.job); err != test.want || result.Text != "" {
			t.Errorf("tool %d: result %q, error %v; want %v", test.job.Tool, result.Text, err, test.want)
		}
	}
}

func check(t *testing.T, job Job, want string) {
	t.Helper()
	if result, err := Run(job); err != nil || result.Text != want {
		t.Fatalf("tool %d: result %q, error %v; want %q", job.Tool, result.Text, err, want)
	}
}
