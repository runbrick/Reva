// Package model holds the plain data types Reqo stores on disk and passes
// between services and the UI. It has no behavior beyond small helpers and
// imports nothing outside the standard library.
//
// Every type serializes to indented JSON with stable field names so that a
// data directory can be kept in Git.
package model

import (
	"strings"
	"time"
)

// SchemaVersion is written into every file Reqo persists.
const SchemaVersion = 1

// Method is an HTTP method.
type Method string

const (
	MethodGet     Method = "GET"
	MethodPost    Method = "POST"
	MethodPut     Method = "PUT"
	MethodPatch   Method = "PATCH"
	MethodDelete  Method = "DELETE"
	MethodHead    Method = "HEAD"
	MethodOptions Method = "OPTIONS"
)

// Methods lists the methods offered in the method picker, in display order.
var Methods = []Method{MethodGet, MethodPost, MethodPut, MethodPatch, MethodDelete, MethodHead, MethodOptions}

// KV is one row of a key/value table (query params, headers, url-encoded
// form fields, variables). Disabled rows are kept but not sent.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	// Description is the user's note for the row; it is never sent.
	Description string `json:"description,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
}

// BodyMode selects how Request.Body is encoded.
type BodyMode string

const (
	BodyNone       BodyMode = "none"
	BodyJSON       BodyMode = "json"
	BodyRaw        BodyMode = "raw"
	BodyFormData   BodyMode = "form-data"
	BodyURLEncoded BodyMode = "urlencoded"
	BodyBinary     BodyMode = "binary"
)

// BodyModes lists the body modes in display order.
var BodyModes = []BodyMode{BodyNone, BodyJSON, BodyRaw, BodyFormData, BodyURLEncoded, BodyBinary}

// FormField is one multipart/form-data part. When File is true, Value is a
// path on the local disk whose contents are sent as the part.
type FormField struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	File     bool   `json:"file,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// Body is a request body. Only the fields that belong to Mode are used.
type Body struct {
	Mode BodyMode `json:"mode"`
	// Text is the body for BodyJSON and BodyRaw.
	Text string `json:"text,omitempty"`
	// ContentType overrides the Content-Type sent for BodyRaw (default text/plain).
	ContentType string      `json:"contentType,omitempty"`
	Form        []FormField `json:"form,omitempty"`
	URLEncoded  []KV        `json:"urlencoded,omitempty"`
	// FilePath is the file sent for BodyBinary.
	FilePath string `json:"filePath,omitempty"`
}

// AuthType selects Request.Auth's scheme.
type AuthType string

const (
	AuthNone   AuthType = "none"
	AuthBearer AuthType = "bearer"
	AuthBasic  AuthType = "basic"
	AuthAPIKey AuthType = "apikey"
)

// AuthTypes lists the auth schemes in display order.
var AuthTypes = []AuthType{AuthNone, AuthBearer, AuthBasic, AuthAPIKey}

// APIKeyIn says where an API key is placed.
type APIKeyIn string

const (
	APIKeyInHeader APIKeyIn = "header"
	APIKeyInQuery  APIKeyIn = "query"
)

// Auth is a request's authentication. Only the fields that belong to Type are used.
type Auth struct {
	Type     AuthType `json:"type"`
	Token    string   `json:"token,omitempty"`    // bearer
	Username string   `json:"username,omitempty"` // basic
	Password string   `json:"password,omitempty"` // basic
	Key      string   `json:"key,omitempty"`      // apikey name
	Value    string   `json:"value,omitempty"`    // apikey value
	In       APIKeyIn `json:"in,omitempty"`       // apikey placement
}

// Request is everything needed to send one HTTP request. Any string field
// may still contain {{variables}}; they are resolved before sending.
type Request struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Method  Method `json:"method"`
	URL     string `json:"url"`
	Params  []KV   `json:"params,omitempty"`
	Headers []KV   `json:"headers,omitempty"`
	Body    Body   `json:"body"`
	Auth    Auth   `json:"auth"`
	// PreScript and TestScript are optional JavaScript run before sending
	// and after the response arrives.
	PreScript  string `json:"preScript,omitempty"`
	TestScript string `json:"testScript,omitempty"`
	// Options are this request's transport settings.
	Options RequestOptions `json:"options,omitzero"`
}

// Toggle is a per-request on/off override of a global setting.
type Toggle string

const (
	ToggleInherit Toggle = ""    // use the global setting
	ToggleOn      Toggle = "on"  // force on
	ToggleOff     Toggle = "off" // force off
)

// RequestOptions are a request's transport settings. Zero values inherit the
// global Settings. The GUI resolves them (Resolve) before handing the request
// to the RequestService, which then always sees TimeoutMs > 0 and explicit
// toggles; a zero value reaching the service means the safe default.
type RequestOptions struct {
	// TimeoutMs bounds the whole send; 0 inherits Settings.RequestTimeoutSec.
	TimeoutMs       int    `json:"timeoutMs,omitempty"`
	FollowRedirects Toggle `json:"followRedirects,omitempty"`
	VerifyTLS       Toggle `json:"verifyTls,omitempty"`
}

// Resolve returns o with every inherited value replaced by the one s gives.
func (o RequestOptions) Resolve(s Settings) RequestOptions {
	if o.TimeoutMs <= 0 {
		sec := s.RequestTimeoutSec
		if sec <= 0 {
			sec = 60
		}
		o.TimeoutMs = sec * 1000
	}
	if o.FollowRedirects == ToggleInherit {
		o.FollowRedirects = ToggleOn
		if s.NoFollowRedirects {
			o.FollowRedirects = ToggleOff
		}
	}
	if o.VerifyTLS == ToggleInherit {
		o.VerifyTLS = ToggleOn
		if s.InsecureTLS {
			o.VerifyTLS = ToggleOff
		}
	}
	return o
}

// NewRequest returns an empty GET request with the given id.
func NewRequest(id string) Request {
	return Request{ID: id, Method: MethodGet, Body: Body{Mode: BodyNone}, Auth: Auth{Type: AuthNone}}
}

// Clone returns a deep copy of r, so that editing the copy's slices does not
// change r.
func (r Request) Clone() Request {
	r.Params = append([]KV(nil), r.Params...)
	r.Headers = append([]KV(nil), r.Headers...)
	r.Body.Form = append([]FormField(nil), r.Body.Form...)
	r.Body.URLEncoded = append([]KV(nil), r.Body.URLEncoded...)
	return r
}

// Response is the result of sending a Request.
type Response struct {
	StatusCode int    `json:"statusCode"`
	Status     string `json:"status"` // e.g. "200 OK"
	Proto      string `json:"proto"`
	Headers    []KV   `json:"headers"`
	Body       []byte `json:"body"`
	// Size is the number of body bytes received, which may exceed len(Body)
	// when Truncated is true.
	Size      int64         `json:"size"`
	Truncated bool          `json:"truncated,omitempty"`
	Duration  time.Duration `json:"duration"`
	// URL is the final URL after variable substitution and redirects.
	URL string `json:"url"`
	// Cookies are the Set-Cookie values the response carried, parsed.
	Cookies []Cookie `json:"cookies,omitempty"`
	// Timeline is how Duration was spent, in order. Phases the transport
	// skipped (a reused connection has no DNS, connect or TLS) are absent.
	Timeline []Phase `json:"timeline,omitempty"`
}

// Cookie is one Set-Cookie of a response.
type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain,omitempty"`
	Path     string    `json:"path,omitempty"`
	Expires  time.Time `json:"expires,omitzero"`
	MaxAge   int       `json:"maxAge,omitempty"`
	HTTPOnly bool      `json:"httpOnly,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	SameSite string    `json:"sameSite,omitempty"`
}

// PhaseKind names a step of sending a request.
type PhaseKind string

const (
	PhaseDNS      PhaseKind = "dns"      // name lookup
	PhaseConnect  PhaseKind = "connect"  // TCP connect
	PhaseTLS      PhaseKind = "tls"      // TLS handshake
	PhaseSend     PhaseKind = "send"     // writing the request
	PhaseWait     PhaseKind = "wait"     // waiting for the first response byte
	PhaseDownload PhaseKind = "download" // reading the body
)

// Phase is how long one step of a send took, and when it started relative
// to the start of the send.
type Phase struct {
	Kind     PhaseKind     `json:"kind"`
	Start    time.Duration `json:"start"`
	Duration time.Duration `json:"duration"`
}

// Header returns the first value of the named response header, case-insensitively.
func (r *Response) Header(name string) string {
	for _, h := range r.Headers {
		if strings.EqualFold(h.Key, name) {
			return h.Value
		}
	}
	return ""
}

// HistoryEntry records one sent request.
type HistoryEntry struct {
	ID         string        `json:"id"`
	At         time.Time     `json:"at"`
	Request    Request       `json:"request"`
	StatusCode int           `json:"statusCode,omitempty"`
	Duration   time.Duration `json:"duration,omitempty"`
	Error      string        `json:"error,omitempty"`
}

// ItemType distinguishes collection folders from requests.
type ItemType string

const (
	ItemFolder  ItemType = "folder"
	ItemRequest ItemType = "request"
)

// Item is a node in a collection: a folder holding more items, or a request.
type Item struct {
	Type    ItemType `json:"type"`
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Request *Request `json:"request,omitempty"`
	Items   []Item   `json:"items,omitempty"`
}

// Collection is a named tree of folders and requests, stored as one file.
type Collection struct {
	Version   int    `json:"version"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Items     []Item `json:"items"`
	Variables []KV   `json:"variables,omitempty"`
}

// Environment is a named set of variables substituted into {{name}} placeholders.
type Environment struct {
	Version   int    `json:"version"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Variables []KV   `json:"variables"`
}

// Settings are the user's preferences.
type Settings struct {
	Version      int    `json:"version"`
	Theme        string `json:"theme"`    // "light", "dark" or "system"
	Language     string `json:"language"` // "zh" or "en"
	ActiveEnvID  string `json:"activeEnvId,omitempty"`
	HistoryLimit int    `json:"historyLimit"`
	// RequestTimeoutSec is the default send timeout; 0 means 60.
	RequestTimeoutSec int `json:"requestTimeoutSec,omitempty"`
	// NoFollowRedirects and InsecureTLS are the global transport defaults;
	// their zero values are the safe ones (follow redirects, verify TLS).
	NoFollowRedirects bool `json:"noFollowRedirects,omitempty"`
	InsecureTLS       bool `json:"insecureTls,omitempty"`
}

// DefaultSettings returns the settings used before the user changes any.
func DefaultSettings() Settings {
	return Settings{Version: SchemaVersion, Theme: "light", Language: "zh", HistoryLimit: 200, RequestTimeoutSec: 60}
}
