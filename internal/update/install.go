package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const maxAssetBytes = 128 << 20

const (
	setup    = "setup"
	portable = "portable"
	bundle   = "bundle"
)

type Offer struct {
	asset  releaseAsset
	target string
	kind   string
}

func (offer Offer) Available() bool { return offer.asset.Name != "" }

func (release Release) Offer() Offer {
	if !Newer(Current, release.Version) {
		return Offer{}
	}
	executable, err := os.Executable()
	if err != nil {
		return Offer{}
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return Offer{}
	}
	target, kind := installation(executable)
	return release.offerFor(runtime.GOOS, runtime.GOARCH, target, kind)
}

func (release Release) offerFor(goos, arch, target, kind string) Offer {
	if target == "" || (arch != "amd64" && arch != "arm64") {
		return Offer{}
	}
	if _, valid := parse(release.Version); !valid {
		return Offer{}
	}
	var suffix string
	switch kind {
	case setup:
		suffix = "-setup.exe"
	case portable, bundle:
		suffix = ".zip"
	default:
		return Offer{}
	}
	name := "reva-" + release.Version + "-" + goos + "-" + arch + suffix
	var offer Offer
	for _, asset := range release.assets {
		if asset.Name != name {
			continue
		}
		if offer.Available() || asset.URL != releaseRoot+"download/v"+release.Version+"/"+name ||
			asset.Size <= 0 || asset.Size > maxAssetBytes || !validDigest(asset.Digest) {
			return Offer{}
		}
		offer = Offer{asset: asset, target: target, kind: kind}
	}
	return offer
}

func validDigest(digest string) bool {
	hexDigest, ok := strings.CutPrefix(digest, "sha256:")
	if !ok || len(hexDigest) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(hexDigest)
	return err == nil
}

func assetRedirect(req *http.Request, via []*http.Request) error {
	if len(via) < 5 && req.URL.Scheme == "https" && req.URL.User == nil {
		switch req.URL.Host {
		case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
			return nil
		}
	}
	return errors.New("update download redirected outside GitHub's release hosts")
}

func download(ctx context.Context, client *http.Client, asset releaseAsset, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Reva/"+Current)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength >= 0 && resp.ContentLength != asset.Size {
		return errors.New("update download has the wrong size")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, asset.Size+1))
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return errors.Join(err, os.Remove(path))
	}
	if written != asset.Size || !strings.EqualFold("sha256:"+hex.EncodeToString(hash.Sum(nil)), asset.Digest) {
		return errors.Join(errors.New("update download did not match the release checksum or size"), os.Remove(path))
	}
	return nil
}

func (offer Offer) Install(ctx context.Context) error {
	if !offer.Available() {
		return errors.New("no compatible update package")
	}
	stage, err := os.MkdirTemp(filepath.Dir(offer.target), stagePrefix)
	if err != nil {
		return err
	}
	handedOff := false
	defer func() {
		if !handedOff {
			_ = os.RemoveAll(stage)
		}
	}()
	artifact := filepath.Join(stage, offer.asset.Name)
	client := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: assetRedirect}
	if err := download(ctx, client, offer.asset, artifact); err != nil {
		return err
	}
	payload := filepath.Join(stage, payloadName(offer.kind))
	if offer.kind == setup {
		err = os.Rename(artifact, payload)
	} else {
		err = unpack(ctx, artifact, payload, offer.kind)
		_ = os.Remove(artifact)
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := handOff(stage, offer.target, offer.kind); err != nil {
		return err
	}
	handedOff = true
	return nil
}
