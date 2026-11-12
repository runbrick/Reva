package gui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/runbrick/reqo/internal/gist"
	"github.com/runbrick/reqo/internal/gui/icons"
	"github.com/runbrick/reqo/internal/s3"
	"github.com/runbrick/reqo/internal/storage"
	"github.com/runbrick/reqo/internal/syncing"
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type syncConfig struct {
	Provider   string `json:"provider"`
	URL        string `json:"url,omitempty"`
	User       string `json:"user,omitempty"`
	File       string `json:"file,omitempty"`
	Region     string `json:"region,omitempty"`
	Bucket     string `json:"bucket,omitempty"`
	AccessKey  string `json:"accessKey,omitempty"`
	SnippetID  string `json:"snippetId,omitempty"`
	APIAddress string `json:"apiAddress,omitempty"`
	PathStyle  bool   `json:"pathStyle,omitempty"`
	Auto       bool   `json:"auto,omitempty"`
}

type syncState struct {
	config                                                                             syncConfig
	serverPassword, backupPassword                                                     string
	editURL, editUser, editFile, editServerPassword, editBackupPassword                string
	editProvider, editRegion, editBucket, editAccessKey, editSnippetID, editAPIAddress string
	editPathStyle                                                                      bool
	open, confirm                                                                      bool
	passwordOpen                                                                       bool
	action                                                                             string
	file                                                                               string
	errorText                                                                          string
	running                                                                            bool
	job                                                                                *syncJob
	effect                                                                             *ui.EffectState[syncResult, *syncJob]
	last                                                                               time.Time
	autoStarted                                                                        bool
	autoFingerprint                                                                    [32]byte
}

type syncJob struct {
	action, file, dir, password string
	server                      syncing.WebDAVConfig
	config                      syncConfig
	local, base                 syncing.Snapshot
}

type syncResult struct {
	action    string
	snapshot  syncing.Snapshot
	base      syncing.Snapshot
	remote    syncing.Snapshot
	missing   bool
	file      string
	snippetID string
}

func (a *App) syncSettings(c ui.C) {
	s := &a.settings.sync
	labels := &a.S.Settings
	if !s.autoStarted {
		if a.settings.dirError == nil {
			var config syncConfig
			if err := storage.ReadJSON(filepath.Join(a.settings.dir, "sync-config.json"), &config); err == nil {
				s.config = config
			} else if !errors.Is(err, os.ErrNotExist) {
				s.errorText = err.Error()
			}
		}
		s.autoStarted = true
	}
	a.scheduleAutoSync(c)
	settingsCard(c, labels.BackupSync, func() {
		ui.Text(c, labels.BackupFile).Bold().MarginTop(Space2)
		settingsRow(c, labels.BackupDescription, "", func() {
			if ui.Button(c, labels.Export).Icon(icons.Download).Outline().Disabled(s.running).Clicked() {
				s.action, s.passwordOpen = "export", true
				s.editBackupPassword, s.errorText = "", ""
			}
			if ui.Button(c, labels.Restore).Icon(icons.Upload).Outline().Disabled(s.running).Clicked() {
				s.action, s.passwordOpen = "restore", true
				s.editBackupPassword, s.errorText = "", ""
			}
		})
		ui.Divider(c)
		ui.Text(c, labels.Sync).Bold().MarginTop(Space2)
		server := labels.NotConfigured
		if s.config.URL != "" {
			server = s.config.URL
		}
		if s.config.Provider == "github" {
			server = labels.GitHub + " " + s.config.SnippetID
		}
		if s.config.Provider == "gitee" {
			server = labels.Gitee + " " + s.config.SnippetID
		}
		settingsRow(c, labels.Server, server, func() {
			for _, provider := range []struct{ id, name string }{{"webdav", labels.WebDAV}, {"s3", labels.S3}, {"github", labels.GitHub}, {"gitee", labels.Gitee}} {
				if ui.Button(c.Key(provider.id), provider.name).Outline().Disabled(s.running).Clicked() {
					s.editProvider = provider.id
					s.editURL, s.editUser, s.editFile = s.config.URL, s.config.User, s.config.File
					s.editRegion, s.editBucket, s.editAccessKey = s.config.Region, s.config.Bucket, s.config.AccessKey
					s.editSnippetID, s.editAPIAddress, s.editPathStyle = s.config.SnippetID, s.config.APIAddress, s.config.PathStyle
					s.editServerPassword, s.editBackupPassword = "", ""
					s.open, s.errorText = true, ""
				}
			}
		})
		ui.Divider(c)
		settingsRow(c, labels.AutoSync, labels.AutoSyncDescription, func() {
			if ui.Switch(c, &s.config.Auto, "").Label(labels.AutoSync).Changed() {
				c.After(func() { a.saveSyncConfig() })
			}
		})
		ui.Divider(c)
		settingsRow(c, labels.Sync, labels.ServerDescription, func() {
			ready := s.config.Provider != "" && len(s.backupPassword) >= 8
			if ui.Button(c, labels.SyncNow).Icon(icons.RefreshCw).Outline().Disabled(!ready || s.running).Clicked() {
				c.After(func() { a.startSync("sync", "") })
			}
			if ui.Button(c, labels.RestoreRemote).Outline().Disabled(!ready || s.running).Clicked() {
				s.confirm = true
			}
		})
		if s.running {
			ui.Text(c, labels.Working).Muted()
		}
		if s.errorText != "" {
			ui.Wrap(c, s.errorText).FillWidth().Color(c.Theme().Danger)
		}
	})
}

func (a *App) saveSyncConfig() {
	s := &a.settings.sync
	if a.settings.dirError != nil {
		s.errorText = a.settings.dirError.Error()
		return
	}
	if err := storage.WriteJSON(filepath.Join(a.settings.dir, "sync-config.json"), s.config); err != nil {
		s.errorText = err.Error()
	}
}

func (a *App) syncDialog(c ui.C) {
	s := &a.settings.sync
	labels := &a.S.Settings
	if s.open {
		ui.ModalWith(c, &s.open, labels.Configure, ui.ModalSpec{Width: 560}, func() {
			ui.Column(c).FillWidth().Gap(Space3).Children(func() {
				switch s.editProvider {
				case "s3":
					ui.TextField(c, &s.editURL, labels.Endpoint, "https://s3.example.com").FillWidth()
					ui.TextField(c, &s.editRegion, labels.Region, "us-east-1").FillWidth()
					ui.TextField(c, &s.editBucket, labels.Bucket, "").FillWidth()
					ui.TextField(c, &s.editFile, labels.ObjectKey, s3.DefaultKey).FillWidth()
					ui.TextField(c, &s.editAccessKey, labels.AccessKey, "").FillWidth()
					ui.TextField(c, &s.editServerPassword, labels.SecretKey, "").FillWidth().Secure()
					ui.Switch(c, &s.editPathStyle, "").Label(labels.PathStyle)
				case "github", "gitee":
					ui.TextField(c, &s.editSnippetID, labels.SnippetID, "").FillWidth()
					ui.TextField(c, &s.editFile, labels.SnippetFile, gist.DefaultFile).FillWidth()
					ui.TextField(c, &s.editAPIAddress, labels.APIAddress, "").FillWidth()
					ui.TextField(c, &s.editServerPassword, labels.ServerPassword, "").FillWidth().Secure()
				default:
					ui.TextField(c, &s.editURL, labels.Address, "https://example.com/dav/").FillWidth()
					ui.TextField(c, &s.editUser, labels.Username, "").FillWidth()
					ui.TextField(c, &s.editFile, labels.RemoteFile, "reqo/backup.reqobak").FillWidth()
					ui.TextField(c, &s.editServerPassword, labels.ServerPassword, "").FillWidth().Secure()
				}
				ui.TextField(c, &s.editBackupPassword, labels.BackupPassword, "").FillWidth().Secure()
				ui.Wrap(c, labels.PasswordHint).FillWidth().Muted()
				if s.errorText != "" {
					ui.Wrap(c, s.errorText).FillWidth().Color(c.Theme().Danger)
				}
			})
		}, func() {
			if ui.Button(c, labels.Cancel).Outline().Clicked() {
				s.open = false
				s.errorText = ""
			}
			if ui.Button(c, labels.Save).Clicked() {
				c.After(func() { a.configureSync() })
			}
		})
	}
	if s.passwordOpen {
		ui.ModalWith(c, &s.passwordOpen, labels.BackupFile, ui.ModalSpec{Width: 480}, func() {
			ui.TextField(c, &s.editBackupPassword, labels.BackupPassword, "").FillWidth().Secure()
			if s.errorText != "" {
				ui.Wrap(c, s.errorText).FillWidth().Color(c.Theme().Danger)
			}
		}, func() {
			if ui.Button(c, labels.Cancel).Outline().Clicked() {
				s.passwordOpen = false
				s.errorText = ""
			}
			if ui.Button(c, labels.Save).Clicked() {
				c.After(func() { a.chooseBackupFile() })
			}
		})
	}
	if s.confirm {
		ui.ModalWith(c, &s.confirm, labels.ConfirmRestore, ui.ModalSpec{Width: 520, Description: labels.RestoreWarning}, func() {}, func() {
			if ui.Button(c, labels.Cancel).Outline().Clicked() {
				s.confirm = false
			}
			if ui.Button(c, labels.RestoreRemote).Clicked() {
				c.After(func() { s.confirm = false; a.startSync("remote-restore", "") })
			}
		})
	}
}

func (a *App) configureSync() {
	s := &a.settings.sync
	if len(s.editBackupPassword) < 8 && len(s.backupPassword) < 8 {
		s.errorText = a.S.Settings.ShortPassword
		return
	}
	secret := s.editServerPassword
	if secret == "" && s.editProvider == s.config.Provider {
		secret = s.serverPassword
	}
	config := syncConfig{Provider: s.editProvider, URL: s.editURL, User: s.editUser, File: s.editFile,
		Region: s.editRegion, Bucket: s.editBucket, AccessKey: s.editAccessKey, PathStyle: s.editPathStyle,
		SnippetID: s.editSnippetID, APIAddress: s.editAPIAddress, Auto: s.config.Auto}
	if _, _, err := makeRemote(config, secret); err != nil {
		s.errorText = err.Error()
		return
	}
	if s.config.Provider != config.Provider || s.config.URL != config.URL || s.config.Bucket != config.Bucket || s.config.SnippetID != config.SnippetID || s.config.File != config.File {
		// A different remote starts with no common baseline.
		if err := os.Remove(filepath.Join(a.settings.dir, "sync-baseline.reqobak")); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.errorText = err.Error()
			return
		}
	}
	s.config, s.serverPassword = config, secret
	if s.editBackupPassword != "" {
		s.backupPassword = s.editBackupPassword
	}
	a.saveSyncConfig()
	if s.errorText == "" {
		s.open = false
	}
}

func (a *App) chooseBackupFile() {
	s := &a.settings.sync
	if len(s.editBackupPassword) < 8 {
		s.errorText = a.S.Settings.ShortPassword
		return
	}
	filter := []app.FileFilter{{Name: a.S.Settings.FileType, Pattern: "*.reqobak"}}
	var path string
	var err error
	if s.action == "export" {
		path, err = app.SaveFile(app.SaveFileOptions{Title: a.S.Settings.Export, SuggestedName: "reqo.reqobak", Filters: filter})
	} else {
		var paths []string
		paths, err = app.OpenFiles(app.OpenFileOptions{Title: a.S.Settings.Restore, Filters: filter})
		if len(paths) > 0 {
			path = paths[0]
		}
	}
	if errors.Is(err, app.ErrCanceled) {
		return
	}
	if err != nil || path == "" {
		s.errorText = a.S.Settings.FileError
		return
	}
	s.backupPassword = s.editBackupPassword
	s.passwordOpen = false
	a.startSync(s.action, path)
}

func (a *App) startSync(action, file string) {
	s := &a.settings.sync
	if s.running {
		return
	}
	if a.settings.dir == "" {
		s.errorText = a.S.Settings.FileError
		return
	}
	if action == "sync" || action == "remote-restore" {
		if s.config.Provider == "" || len(s.backupPassword) < 8 {
			s.errorText = a.S.Settings.NotConfigured
			return
		}
	}
	base, err := syncing.Baseline(a.settings.dir, s.backupPassword)
	if err != nil {
		s.errorText = err.Error()
		return
	}
	local := syncing.Capture(a.Svc)
	if action == "sync" {
		local = syncing.Shared(a.Svc)
	}
	s.job = &syncJob{action: action, file: file, dir: a.settings.dir, password: s.backupPassword,
		config: s.config,
		server: syncing.WebDAVConfig{URL: s.config.URL, User: s.config.User, File: s.config.File, Password: s.serverPassword},
		local:  local, base: base}
	s.effect = ui.NewEffectKeyed(runSyncJob)
	s.running, s.errorText = true, ""
}

func runSyncJob(ctx context.Context, job *syncJob) (syncResult, error) {
	result := syncResult{action: job.action, file: job.file, base: job.base}
	switch job.action {
	case "export":
		data, err := syncing.Seal(job.local, job.password)
		if err != nil {
			return result, err
		}
		return result, storage.WriteFileAtomic(job.file, data)
	case "restore":
		data, err := os.ReadFile(job.file)
		if err != nil {
			return result, err
		}
		result.snapshot, err = syncing.Open(data, job.password)
		return result, err
	}
	server, snippet, err := makeRemote(job.config, job.server.Password)
	if err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	remote, tag, missing, err := syncing.Download(ctx, server, job.password)
	if err != nil {
		return result, err
	}
	result.remote, result.missing = remote, missing
	if job.action == "remote-restore" {
		if missing {
			return result, os.ErrNotExist
		}
		result.snapshot = remote
		return result, nil
	}
	result.snapshot = syncing.Merge(job.base, job.local, remote, time.Now())
	merged, _ := json.Marshal(result.snapshot)
	original, _ := json.Marshal(remote)
	if missing || string(merged) != string(original) {
		if err := syncing.Upload(ctx, server, result.snapshot, job.password, tag); err != nil {
			return result, err
		}
	}
	if snippet != nil {
		result.snippetID = snippet.ID()
	}
	return result, nil
}

func makeRemote(config syncConfig, secret string) (syncing.Remote, *gist.Client, error) {
	switch config.Provider {
	case "s3":
		store, err := s3.New(s3.Config{Endpoint: config.URL, Region: config.Region, Bucket: config.Bucket,
			Key: defaultSyncFile(config.File, s3.DefaultKey), AccessKey: config.AccessKey, Secret: secret, PathStyle: config.PathStyle}, nil)
		return store, nil, err
	case "github", "gitee":
		provider := gist.GitHub
		if config.Provider == "gitee" {
			provider = gist.Gitee
		}
		store, err := gist.New(gist.Config{Provider: provider, Token: secret, ID: config.SnippetID,
			File: defaultSyncFile(config.File, gist.DefaultFile), APIBase: config.APIAddress}, nil)
		return store, store, err
	case "webdav":
		store, err := syncing.NewWebDAV(syncing.WebDAVConfig{URL: config.URL, User: config.User, File: config.File, Password: secret})
		return store, nil, err
	default:
		return nil, nil, errors.New("unsupported sync provider")
	}
}

func defaultSyncFile(file, fallback string) string {
	if file == "" {
		return fallback
	}
	return file
}

func (a *App) declareSync(c ui.C) {
	s := &a.settings.sync
	a.scheduleAutoSync(c)
	if !s.running || s.effect == nil {
		return
	}
	ui.Effect(c, s.effect, s.job)
	if result, err, ok := s.effect.Take(); ok {
		s.running, s.effect = false, nil
		if err == nil {
			err = a.finishSync(result)
		}
		if err != nil {
			message := err.Error()
			if s.serverPassword != "" {
				message = strings.ReplaceAll(message, s.serverPassword, "***")
				message = strings.ReplaceAll(message, url.QueryEscape(s.serverPassword), "***")
			}
			s.errorText = message
			a.Toast(a.S.Settings.SyncError, message, ui.ToastError)
			return
		}
		message := a.S.Settings.SyncDone
		if result.action == "export" {
			message = a.S.Settings.BackupDone
		}
		if result.action == "restore" || result.action == "remote-restore" {
			message = a.S.Settings.RestoreDone
		}
		a.Toast(message, "", ui.ToastSuccess)
	}
}

func (a *App) scheduleAutoSync(c ui.C) {
	s := &a.settings.sync
	if !s.running && s.autoStarted && s.config.Auto && s.config.Provider != "" && len(s.backupPassword) >= 8 {
		fingerprint := syncFingerprint(syncing.Shared(a.Svc))
		if fingerprint != s.autoFingerprint {
			s.autoFingerprint = fingerprint
			c.After(func() { a.startSync("sync", "") })
		}
	}
}

func (a *App) finishSync(result syncResult) error {
	s := &a.settings.sync
	if result.action == "export" {
		return nil
	}
	if result.action == "sync" {
		current, _ := json.Marshal(syncing.Shared(a.Svc))
		original, _ := json.Marshal(s.job.local)
		if string(current) != string(original) {
			return errors.New(a.S.Settings.LocalChanges)
		}
	}
	previous := syncing.Capture(a.Svc)
	data, err := syncing.Seal(previous, s.backupPassword)
	if err != nil {
		return err
	}
	name := "before-sync.reqobak"
	if result.action == "remote-restore" || result.action == "restore" {
		name = "before-restore.reqobak"
	}
	if err := storage.WriteFileAtomic(filepath.Join(a.settings.dir, name), data); err != nil {
		return err
	}
	toApply := result.snapshot
	if result.action == "sync" || result.action == "remote-restore" {
		settings := a.Svc.Settings.Get()
		toApply.Settings.Theme, toApply.Settings.Language, toApply.Settings.ActiveEnvID = settings.Theme, settings.Language, settings.ActiveEnvID
		found := toApply.Settings.ActiveEnvID == ""
		for _, env := range toApply.Environments {
			found = found || env.ID == toApply.Settings.ActiveEnvID
		}
		if !found {
			toApply.Settings.ActiveEnvID = ""
		}
	}
	before, _ := json.Marshal(previous)
	after, _ := json.Marshal(toApply)
	if !bytes.Equal(before, after) {
		if err := syncing.Apply(a.Svc, toApply); err != nil {
			return errors.Join(err, syncing.Apply(a.Svc, previous))
		}
	}
	priorCollections, _ := json.Marshal(previous.Collections)
	nextCollections, _ := json.Marshal(toApply.Collections)
	if result.action == "remote-restore" || result.action == "restore" || !bytes.Equal(priorCollections, nextCollections) {
		a.tabs = nil
		a.active = 0
		a.NewTab()
	}
	if result.action == "sync" || result.action == "remote-restore" {
		if err := syncing.SaveBaseline(a.settings.dir, result.snapshot, s.backupPassword); err != nil {
			return err
		}
		if result.snippetID != "" && s.config.SnippetID == "" {
			s.config.SnippetID = result.snippetID
			a.saveSyncConfig()
		}
	}
	a.ApplySettings(a.Svc.Settings.Get())
	s.autoFingerprint = syncFingerprint(syncing.Shared(a.Svc))
	s.last = time.Now()
	return nil
}

func syncFingerprint(snapshot syncing.Snapshot) [32]byte {
	data, _ := json.Marshal(snapshot)
	return sha256.Sum256(data)
}
