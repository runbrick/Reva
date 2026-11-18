package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/storage"
)

const (
	maxFileBytes = 32 << 20
	maxJSONDepth = 1024
	maxItemDepth = 256
	maxItems     = 10000
	maxRows      = 100000
	// Bound both Resolve's milliseconds multiplication and HTTP time.Duration.
	maxTimeoutMS  = min(int64((1<<63-1)/int64(time.Millisecond)), int64(^uint(0)>>1))
	maxTimeoutSec = maxTimeoutMS / 1000
)

var errInvalid = errors.New("invalid native data")

type runInput struct {
	collection  model.Collection
	settings    model.Settings
	environment []model.KV
}

// storage.Dir creates missing directories. Mirror only its selection rules;
// neither this resolver nor the loader creates any data directories or files.
func readOnlyDir(explicit string) (string, error) {
	dir := explicit
	if dir == "" {
		dir = os.Getenv(storage.EnvHome)
	}
	if dir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(base, "Reva")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errInvalid
	}
	return dir, nil
}

func loadInput(dir, collectionID, environmentID string) (runInput, string) {
	in := runInput{settings: model.DefaultSettings()}
	err := readNative(filepath.Join(dir, "settings.json"), &in.settings)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return runInput{}, "settings_invalid_or_unreadable"
	}
	if in.settings.Version != model.SchemaVersion || in.settings.RequestTimeoutSec < 0 || int64(in.settings.RequestTimeoutSec) > maxTimeoutSec {
		return runInput{}, "settings_invalid_or_unreadable"
	}
	if err := readNative(filepath.Join(dir, "collections", collectionID+".json"), &in.collection); err != nil ||
		in.collection.Version != model.SchemaVersion || in.collection.ID != collectionID || !validCollection(in.collection) {
		return runInput{}, "collection_invalid_or_unreadable"
	}
	if environmentID != "" {
		env, local := in.collection.EnvironmentScope.FindLocal(environmentID)
		if !local {
			if err := readNative(filepath.Join(dir, "environments", environmentID+".json"), &env); err != nil ||
				env.Version != model.SchemaVersion || env.ID != environmentID || strings.TrimSpace(env.Name) == "" || len(env.Variables) > maxRows {
				return runInput{}, "environment_invalid_or_unreadable"
			}
		}
		in.environment = env.Variables
	}
	return in, ""
}

// IDs are filenames, so disallow path/stream syntax on every host, including
// Windows reserved names when running on Unix. IDs never appear in diagnostics.
func validID(id string) bool {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\:<>"|?*`+"\x00") ||
		strings.HasSuffix(id, ".") || strings.HasSuffix(id, " ") {
		return false
	}
	for _, r := range id {
		if r < 32 {
			return false
		}
	}
	base := strings.ToUpper(strings.SplitN(id, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" ||
		(len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
		return false
	}
	return true
}

func validCollection(col model.Collection) bool {
	if strings.TrimSpace(col.Name) == "" {
		return false
	}
	seen := make(map[string]bool)
	items, rows := 0, len(col.Variables)
	if scope := col.EnvironmentScope; scope != nil {
		rows += len(scope.VisibleIDs) + len(scope.Local)
		envIDs := make(map[string]bool, len(scope.Local))
		for _, env := range scope.Local {
			if !validID(env.ID) || envIDs[env.ID] || env.Version != model.SchemaVersion || strings.TrimSpace(env.Name) == "" {
				return false
			}
			envIDs[env.ID] = true
			rows += len(env.Variables)
			if rows > maxRows {
				return false
			}
		}
	}
	var walk func([]model.Item, int) bool
	walk = func(nodes []model.Item, depth int) bool {
		if len(nodes) != 0 && depth > maxItemDepth {
			return false
		}
		for _, item := range nodes {
			items++
			if items > maxItems || !validID(item.ID) || seen[item.ID] || strings.TrimSpace(item.Name) == "" {
				return false
			}
			seen[item.ID] = true
			switch item.Type {
			case model.ItemFolder:
				if item.Request != nil || !walk(item.Items, depth+1) {
					return false
				}
			case model.ItemRequest:
				if item.Request == nil || len(item.Items) != 0 || strings.TrimSpace(item.Request.Name) == "" {
					return false
				}
				req := item.Request
				rows += len(req.Params) + len(req.Headers) + len(req.Body.Form) + len(req.Body.URLEncoded)
				if req.Options.TimeoutMs < 0 || int64(req.Options.TimeoutMs) > maxTimeoutMS ||
					!validToggle(req.Options.FollowRedirects) || !validToggle(req.Options.VerifyTLS) {
					return false
				}
			default:
				return false
			}
			if rows > maxRows {
				return false
			}
		}
		return rows <= maxRows
	}
	return walk(col.Items, 1)
}

func validToggle(t model.Toggle) bool {
	return t == model.ToggleInherit || t == model.ToggleOn || t == model.ToggleOff
}

// readNative bounds the actual read even if a file grows, rejects non-regular
// files, and requires an exact native schema (unknown/duplicate/case-aliased
// fields and scalar nulls are errors). It never returns sensitive errors to UI.
func readNative(path string, dst any) error {
	// Check before Open as well: opening a named pipe could block before Fstat.
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return errInvalid
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return errInvalid
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil || len(data) > maxFileBytes || !utf8.Valid(data) {
		return errInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := checkValue(dec, reflect.TypeOf(dst).Elem(), 0); err != nil {
		return errInvalid
	}
	if _, err := dec.Token(); err != io.EOF {
		return errInvalid
	}
	// Decode only after structural checks. Missing version must not inherit the
	// initialized defaults, so always decode into a fresh zero value.
	fresh := reflect.New(reflect.TypeOf(dst).Elem())
	if err := json.Unmarshal(data, fresh.Interface()); err != nil {
		return errInvalid
	}
	reflect.ValueOf(dst).Elem().Set(fresh.Elem())
	return nil
}

func checkValue(dec *json.Decoder, typ reflect.Type, depth int) error {
	if depth > maxJSONDepth {
		return errInvalid
	}
	token, err := dec.Token()
	if err != nil {
		return errInvalid
	}
	if token == nil {
		if typ.Kind() == reflect.Slice || typ.Kind() == reflect.Pointer {
			return nil
		}
		return errInvalid
	}
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Struct:
		if token != json.Delim('{') {
			return errInvalid
		}
		fields := make(map[string]reflect.Type, typ.NumField())
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
			fields[name] = field.Type
		}
		seen := make(map[string]bool)
		for dec.More() {
			key, err := dec.Token()
			if err != nil {
				return errInvalid
			}
			name, ok := key.(string)
			field, exists := fields[name]
			if !ok || !exists || seen[name] {
				return errInvalid
			}
			seen[name] = true
			if err := checkValue(dec, field, depth+1); err != nil {
				return err
			}
		}
		for _, name := range requiredFields(typ) {
			if !seen[name] {
				return errInvalid
			}
		}
		return closeJSON(dec, '}')
	case reflect.Slice:
		if token != json.Delim('[') {
			return errInvalid
		}
		for dec.More() {
			if err := checkValue(dec, typ.Elem(), depth+1); err != nil {
				return err
			}
		}
		return closeJSON(dec, ']')
	case reflect.String:
		if _, ok := token.(string); ok {
			return nil
		}
	case reflect.Bool:
		if _, ok := token.(bool); ok {
			return nil
		}
	case reflect.Int:
		if number, ok := token.(json.Number); ok {
			_, err := number.Int64()
			return err
		}
	}
	return errInvalid
}

func requiredFields(typ reflect.Type) []string {
	switch typ {
	case reflect.TypeFor[model.Collection]():
		return []string{"version", "id", "name", "items"}
	case reflect.TypeFor[model.Environment]():
		return []string{"version", "id", "name", "variables"}
	case reflect.TypeFor[model.Settings]():
		return []string{"version"}
	case reflect.TypeFor[model.Item]():
		return []string{"type", "id", "name"}
	case reflect.TypeFor[model.Request]():
		return []string{"name", "method", "url", "body"}
	case reflect.TypeFor[model.Body]():
		return []string{"mode"}
	case reflect.TypeFor[model.KV](), reflect.TypeFor[model.FormField]():
		return []string{"key", "value"}
	default:
		return nil
	}
}

func closeJSON(dec *json.Decoder, delimiter rune) error {
	token, err := dec.Token()
	if err != nil || token != json.Delim(delimiter) {
		return errInvalid
	}
	return nil
}
