// Package syncing stores an encrypted snapshot of Reva data in one remote file.
package syncing

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/runbrick/reva/internal/cloudsync"
	"github.com/runbrick/reva/internal/model"
	"github.com/runbrick/reva/internal/service"
	"github.com/runbrick/reva/internal/storage"
)

const MaxBytes = 32 << 20
const iterations = 600000

var ErrPassword = errors.New("incorrect password or damaged backup")
var ErrConflict = errors.New("remote data changed; retry synchronization")

// Snapshot contains data that is portable across computers. The local baseline
// and connection credentials are deliberately excluded.
type Snapshot struct {
	Version      int                  `json:"version"`
	Collections  []model.Collection   `json:"collections"`
	Environments []model.Environment  `json:"environments"`
	Settings     model.Settings       `json:"settings"`
	History      []model.HistoryEntry `json:"history"`
	Deleted      map[string]int64     `json:"deleted,omitempty"`
}

type envelope struct {
	Format string `json:"format"`
	Salt   []byte `json:"salt"`
	Nonce  []byte `json:"nonce"`
	Data   []byte `json:"data"`
}

func Seal(snapshot Snapshot, password string) ([]byte, error) {
	if len(password) < 8 {
		return nil, errors.New("backup password must contain at least 8 characters")
	}
	plain, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	if len(plain) > MaxBytes/2 {
		return nil, errors.New("backup exceeds size limit")
	}
	file := envelope{Format: "reva-backup-v1", Salt: make([]byte, 16), Nonce: make([]byte, 12)}
	if _, err = rand.Read(file.Salt); err != nil {
		return nil, err
	}
	if _, err = rand.Read(file.Nonce); err != nil {
		return nil, err
	}
	key, err := pbkdf2.Key(sha256.New, password, file.Salt, iterations, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	file.Data = gcm.Seal(nil, file.Nonce, plain, []byte(file.Format))
	return json.Marshal(file)
}

func Open(data []byte, password string) (Snapshot, error) {
	if len(data) > MaxBytes+1<<16 {
		return Snapshot{}, errors.New("backup exceeds size limit")
	}
	var file envelope
	if json.Unmarshal(data, &file) != nil || file.Format != "reva-backup-v1" || len(file.Salt) != 16 || len(file.Nonce) != 12 || len(file.Data) > MaxBytes {
		return Snapshot{}, errors.New("invalid or unsupported Reva backup")
	}
	key, err := pbkdf2.Key(sha256.New, password, file.Salt, iterations, 32)
	if err != nil {
		return Snapshot{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return Snapshot{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Snapshot{}, err
	}
	plain, err := gcm.Open(nil, file.Nonce, file.Data, []byte(file.Format))
	if err != nil {
		return Snapshot{}, ErrPassword
	}
	var snapshot Snapshot
	if err := json.Unmarshal(plain, &snapshot); err != nil || snapshot.Version != 1 {
		return Snapshot{}, errors.New("invalid or unsupported Reva backup")
	}
	return snapshot, nil
}

func Capture(svc service.Services) Snapshot {
	s := Snapshot{Version: 1, Collections: svc.Collections.List(), Environments: svc.Environments.List(),
		Settings: svc.Settings.Get(), History: svc.History.List()}
	order(&s)
	return s
}

// Shared omits preferences tied to one computer from cloud synchronization.
// A manually exported file still uses Capture and retains them.
func Shared(svc service.Services) Snapshot {
	s := Capture(svc)
	s.Settings.Theme, s.Settings.Language, s.Settings.ActiveEnvID = "", "", ""
	return s
}

// Apply replaces the portable data through the live services, so their caches
// and the disk remain consistent. Existing entities are never removed until
// all incoming entities have been saved successfully.
func Apply(svc service.Services, next Snapshot) error {
	if next.Version != 1 {
		return errors.New("unsupported backup version")
	}
	for _, col := range next.Collections {
		if err := svc.Collections.Save(col); err != nil {
			return err
		}
	}
	for _, env := range next.Environments {
		if err := svc.Environments.Save(env); err != nil {
			return err
		}
	}
	cols := map[string]bool{}
	for _, col := range next.Collections {
		cols[col.ID] = true
	}
	for _, col := range svc.Collections.List() {
		if !cols[col.ID] {
			if err := svc.Collections.Delete(col.ID); err != nil {
				return err
			}
		}
	}
	envs := map[string]bool{}
	for _, env := range next.Environments {
		envs[env.ID] = true
	}
	for _, env := range svc.Environments.List() {
		if !envs[env.ID] {
			if err := svc.Environments.Delete(env.ID); err != nil {
				return err
			}
		}
	}
	if err := svc.Settings.Save(next.Settings); err != nil {
		return err
	}
	if err := svc.History.Clear(); err != nil {
		return err
	}
	for i := len(next.History) - 1; i >= 0; i-- {
		if err := svc.History.Add(next.History[i]); err != nil {
			return err
		}
	}
	return nil
}

func entries(s Snapshot) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for _, col := range s.Collections {
		out["c:"+col.ID], _ = json.Marshal(col)
	}
	for _, env := range s.Environments {
		out["e:"+env.ID], _ = json.Marshal(env)
	}
	for _, history := range s.History {
		out["h:"+history.ID], _ = json.Marshal(history)
	}
	return out
}

// Merge uses the last successful remote snapshot as a three-way baseline.
// Conflicting edits choose the same winner on every device, by content hash.
// Deletions are remembered for 90 days, including on the remote snapshot.
func Merge(base, local, remote Snapshot, now time.Time) Snapshot {
	result := local
	result.Version = 1
	result.Collections, result.Environments, result.History = nil, nil, nil
	result.Deleted = map[string]int64{}
	cutoff := now.Add(-90 * 24 * time.Hour).Unix()
	for _, source := range []map[string]int64{local.Deleted, remote.Deleted} {
		for id, stamp := range source {
			if stamp >= cutoff && stamp > result.Deleted[id] {
				result.Deleted[id] = stamp
			}
		}
	}
	b, l, r := entries(base), entries(local), entries(remote)
	keys := map[string]bool{}
	for _, m := range []map[string]json.RawMessage{b, l, r} {
		for key := range m {
			keys[key] = true
		}
	}
	for key := range keys {
		lb, lok := l[key]
		rb, rok := r[key]
		bb, hadBase := b[key]
		if !hadBase && result.Deleted[key] != 0 {
			continue
		}
		localChanged, remoteChanged := !slices.Equal(lb, bb) || lok != hadBase, !slices.Equal(rb, bb) || rok != hadBase
		if hadBase && !lok && localChanged && result.Deleted[key] == 0 {
			result.Deleted[key] = now.Unix()
		}
		if hadBase && !rok && remoteChanged && result.Deleted[key] == 0 {
			result.Deleted[key] = now.Unix()
		}
		var selected json.RawMessage
		switch {
		case !localChanged:
			selected = rb
		case !remoteChanged:
			selected = lb
		case lok && rok:
			lh, rh := sha256.Sum256(lb), sha256.Sum256(rb)
			selected = lb
			if slices.Compare(lh[:], rh[:]) < 0 {
				selected = rb
			}
		case !lok && !rok:
		default:
			// A deletion and an edit in the same round preserve the edit.
			if lok {
				selected = lb
			} else {
				selected = rb
			}
		}
		if len(selected) == 0 {
			continue
		}
		delete(result.Deleted, key)
		switch key[:2] {
		case "c:":
			var col model.Collection
			if json.Unmarshal(selected, &col) == nil {
				var old, here, there model.Collection
				_ = json.Unmarshal(b[key], &old)
				_ = json.Unmarshal(l[key], &here)
				_ = json.Unmarshal(r[key], &there)
				if len(l[key]) > 0 && len(r[key]) > 0 {
					metadata := func(c model.Collection) []byte {
						c.Items = nil
						data, _ := json.Marshal(c)
						return data
					}
					oldMeta, localMeta, remoteMeta := metadata(old), metadata(here), metadata(there)
					if slices.Equal(remoteMeta, oldMeta) && !slices.Equal(localMeta, oldMeta) {
						col.Name, col.Variables, col.Auth = here.Name, here.Variables, here.Auth
					} else if slices.Equal(localMeta, oldMeta) && !slices.Equal(remoteMeta, oldMeta) {
						col.Name, col.Variables, col.Auth = there.Name, there.Variables, there.Auth
					}
				}
				col.Items = mergeItems("i:"+col.ID+":", old.Items, here.Items, there.Items, result.Deleted, now)
				result.Collections = append(result.Collections, col)
			}
		case "e:":
			var env model.Environment
			if json.Unmarshal(selected, &env) == nil {
				result.Environments = append(result.Environments, env)
			}
		case "h:":
			var history model.HistoryEntry
			if json.Unmarshal(selected, &history) == nil {
				result.History = append(result.History, history)
			}
		}
	}
	// Shared settings take the remote value only if local settings did not change.
	if (local.Settings == base.Settings || base.Settings.Version == 0) && remote.Settings.Version != 0 && remote.Settings != base.Settings {
		result.Settings = remote.Settings
	}
	order(&result)
	return result
}

// mergeItems keeps independent edits inside the same collection, including
// nested folders, without requiring model fields for revision timestamps.
func mergeItems(prefix string, base, local, remote []model.Item, deleted map[string]int64, now time.Time) []model.Item {
	byID := func(items []model.Item) map[string]model.Item {
		out := make(map[string]model.Item, len(items))
		for _, item := range items {
			out[item.ID] = item
		}
		return out
	}
	b, l, r := byID(base), byID(local), byID(remote)
	ids := map[string]bool{}
	for _, source := range []map[string]model.Item{b, l, r} {
		for id := range source {
			ids[id] = true
		}
	}
	// Keep the preferred collection's order where possible, then add records
	// introduced by the other device in stable ID order.
	ordered := make([]string, 0, len(ids))
	for _, list := range [][]model.Item{local, remote, base} {
		for _, item := range list {
			if ids[item.ID] {
				ordered = append(ordered, item.ID)
				delete(ids, item.ID)
			}
		}
	}
	var result []model.Item
	for _, id := range ordered {
		old, hadBase := b[id]
		here, hasLocal := l[id]
		there, hasRemote := r[id]
		key := prefix + id
		if !hadBase && deleted[key] != 0 {
			continue
		}
		plain := func(item model.Item) []byte {
			item.Items = nil
			data, _ := json.Marshal(item)
			return data
		}
		oldData, localData, remoteData := plain(old), plain(here), plain(there)
		localChanged := hadBase != hasLocal || hasLocal && !slices.Equal(oldData, localData)
		remoteChanged := hadBase != hasRemote || hasRemote && !slices.Equal(oldData, remoteData)
		if hadBase && (!hasLocal || !hasRemote) && deleted[key] == 0 {
			deleted[key] = now.Unix()
		}
		var chosen model.Item
		keep := false
		switch {
		case !localChanged:
			chosen, keep = there, hasRemote
		case !remoteChanged:
			chosen, keep = here, hasLocal
		case hasLocal && hasRemote:
			chosen, keep = here, true
			lh, rh := sha256.Sum256(localData), sha256.Sum256(remoteData)
			if slices.Compare(lh[:], rh[:]) < 0 {
				chosen = there
			}
		case hasLocal:
			chosen, keep = here, true
		case hasRemote:
			chosen, keep = there, true
		}
		if !keep {
			continue
		}
		delete(deleted, key)
		chosen.Items = mergeItems(prefix+id+":", old.Items, here.Items, there.Items, deleted, now)
		result = append(result, chosen)
	}
	return result
}

func order(s *Snapshot) {
	slices.SortFunc(s.Collections, func(a, b model.Collection) int { return cmpString(a.ID, b.ID) })
	slices.SortFunc(s.Environments, func(a, b model.Environment) int { return cmpString(a.ID, b.ID) })
	slices.SortFunc(s.History, func(a, b model.HistoryEntry) int {
		if n := b.At.Compare(a.At); n != 0 {
			return n
		}
		return cmpString(a.ID, b.ID)
	})
}

func cmpString(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

type Remote interface {
	Get(context.Context) ([]byte, string, error)
	Put(context.Context, []byte, string) (string, error)
}

// Download and Upload keep remote I/O separate from the live service writes.
func Download(ctx context.Context, remote Remote, password string) (Snapshot, string, bool, error) {
	data, tag, err := remote.Get(ctx)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, cloudsync.ErrNotFound) {
		return Snapshot{Version: 1}, "", true, nil
	}
	if err != nil {
		return Snapshot{}, "", false, err
	}
	snapshot, err := Open(data, password)
	return snapshot, tag, false, err
}

func Upload(ctx context.Context, remote Remote, snapshot Snapshot, password, tag string) error {
	data, err := Seal(snapshot, password)
	if err != nil {
		return err
	}
	_, err = remote.Put(ctx, data, tag)
	if errors.Is(err, cloudsync.ErrChanged) {
		return ErrConflict
	}
	return err
}

// Baseline is the last remote state successfully merged on this device.
func Baseline(dir, password string) (Snapshot, error) {
	data, err := os.ReadFile(filepath.Join(dir, "sync-baseline.revabak"))
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{Version: 1}, nil
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("read sync baseline: %w", err)
	}
	return Open(data, password)
}

func SaveBaseline(dir string, snapshot Snapshot, password string) error {
	data, err := Seal(snapshot, password)
	if err != nil {
		return err
	}
	return storage.WriteFileAtomic(filepath.Join(dir, "sync-baseline.revabak"), data)
}
