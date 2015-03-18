package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	tuf "github.com/flynn/flynn/Godeps/_workspace/src/github.com/flynn/go-tuf/client"
	tufdata "github.com/flynn/flynn/Godeps/_workspace/src/github.com/flynn/go-tuf/data"
	"github.com/flynn/flynn/Godeps/_workspace/src/github.com/inconshreveable/go-update"
	"github.com/flynn/flynn/Godeps/_workspace/src/github.com/kardianos/osext"
	"github.com/flynn/flynn/pkg/random"
	"github.com/flynn/flynn/pkg/version"
)

const upcktimePath = "cktime"

var updateDir = filepath.Join(homedir(), ".flynn", "update")
var updater *Updater

func runUpdate() error {
	if updater == nil || !version.Tagged() {
		return errors.New("Dev builds don't support auto-updates")
	}
	return updater.update()
}

type Updater struct {
	repo     string
	rootKeys []*tufdata.Key
}

func (u *Updater) backgroundRun() {
	if u == nil {
		return
	}
	if !u.wantUpdate() {
		return
	}
	self, err := osext.Executable()
	if err != nil {
		// fail update, couldn't figure out path to self
		return
	}
	// TODO(titanous): logger isn't on Windows. Replace with proper error reports.
	l := exec.Command("logger", "-tflynn")
	c := exec.Command(self, "update")
	if w, err := l.StdinPipe(); err == nil && l.Start() == nil {
		c.Stdout = w
		c.Stderr = w
	}
	c.Start()
}

func (u *Updater) wantUpdate() bool {
	path := filepath.Join(updateDir, upcktimePath)
	if !version.Tagged() || readTime(path).After(time.Now()) {
		return false
	}
	wait := 12*time.Hour + randDuration(8*time.Hour)
	return writeTime(path, time.Now().Add(wait))
}

func (u *Updater) update() error {
	up := update.New()
	if err := up.CanUpdate(); err != nil {
		return err
	}

	if err := os.MkdirAll(updateDir, 0755); err != nil {
		return err
	}
	local, err := tuf.FileLocalStore(filepath.Join(updateDir, "tuf.db"))
	if err != nil {
		return err
	}
	plat := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	opts := &tuf.HTTPRemoteOptions{
		UserAgent: fmt.Sprintf("flynn-cli/%s %s", version.String(), plat),
	}
	remote, err := tuf.HTTPRemoteStore(u.repo, opts)
	if err != nil {
		return err
	}
	client := tuf.NewClient(local, remote)
	if err := u.updateTUFClient(client); err != nil {
		return err
	}
	targets, err := client.Targets()
	if err != nil {
		return err
	}

	name := fmt.Sprintf("/flynn-%s.gz", plat)
	target, ok := targets[name]
	if !ok {
		return fmt.Errorf("missing %q in tuf targets", name)
	}
	if target.Custom == nil || len(*target.Custom) == 0 {
		return errors.New("missing custom metadata in tuf target")
	}
	var data struct {
		Version string
	}
	json.Unmarshal(*target.Custom, &data)
	if data.Version == "" {
		return errors.New("missing version in tuf target")
	}
	if data.Version == version.String() {
		return nil
	}

	bin := &tufBuffer{}
	if err := client.Download(name, bin); err != nil {
		return err
	}
	gr, err := gzip.NewReader(bin)
	if err != nil {
		return err
	}

	err, errRecover := up.FromStream(gr)
	if errRecover != nil {
		return fmt.Errorf("update and recovery errors: %q %q", err, errRecover)
	}
	if err != nil {
		return err
	}
	log.Printf("Updated %s -> %s.", version.String(), data.Version)
	return nil
}

// updateTUFClient updates the given client, initializing and re-running the
// update if ErrNoRootKeys is returned.
func (u *Updater) updateTUFClient(client *tuf.Client) error {
	_, err := client.Update()
	if err == nil || tuf.IsLatestSnapshot(err) {
		return nil
	}
	if err == tuf.ErrNoRootKeys {
		if err := client.Init(u.rootKeys, len(u.rootKeys)); err != nil {
			return err
		}
		return u.updateTUFClient(client)
	}
	return err
}

// returns a random duration in [0,n).
func randDuration(n time.Duration) time.Duration {
	return time.Duration(random.Math.Int63n(int64(n)))
}

func readTime(path string) time.Time {
	p, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return time.Time{}
	}
	if err != nil {
		return time.Now().Add(1000 * time.Hour)
	}
	t, err := time.Parse(time.RFC3339, string(p))
	if err != nil {
		return time.Now().Add(1000 * time.Hour)
	}
	return t
}

func writeTime(path string, t time.Time) bool {
	return ioutil.WriteFile(path, []byte(t.Format(time.RFC3339)), 0644) == nil
}

type tufBuffer struct {
	bytes.Buffer
}

func (b *tufBuffer) Delete() error {
	b.Reset()
	return nil
}
