package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/sreenivas-sadhu-prabhakara/epcg-advance-authorisation-export-obligation/internal/eo"
)

// fileFormatVersion is written into the data file so a later version of the
// tool can tell what it is reading.
const fileFormatVersion = 1

// maxLicences caps how many licences the file will hold, so that a runaway
// script cannot fill a disk. A busy exporter has a few dozen live
// authorisations.
const maxLicences = 500

// fileData is the whole contents of the data file.
type fileData struct {
	Version int `json:"version"`

	// DefaultPolicy is the set of figures a NEW licence starts with. Each
	// licence then keeps its own copy, because an authorisation issued under
	// one edition of the Foreign Trade Policy goes on being governed by the
	// figures that applied to it after the Policy is amended. Changing the
	// default here does not reach back into a licence already saved, and that
	// is deliberate.
	DefaultPolicy eo.Policy `json:"defaultPolicy"`

	Licences []eo.Licence `json:"licences"`
}

// store keeps the licences in memory and writes the whole file out after every
// change. A few hundred licences is a few megabytes, so rewriting the file is
// both simpler and safer than updating it in place.
type store struct {
	mu   sync.RWMutex
	path string
	data fileData
}

// openStore reads the data file, or starts empty if there is no file yet. It
// does not create the file until something is saved.
func openStore(path string) (*store, error) {
	s := &store{
		path: path,
		data: fileData{Version: fileFormatVersion, DefaultPolicy: eo.AsPublished()},
	}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return s, nil
	}

	var loaded fileData
	if err := json.Unmarshal(raw, &loaded); err != nil {
		return nil, fmt.Errorf("reading %s: %w (move the file aside to start again)", path, err)
	}
	if loaded.Version > fileFormatVersion {
		return nil, fmt.Errorf("%s was written by a newer version of this tool (format %d, this build reads %d)",
			path, loaded.Version, fileFormatVersion)
	}
	loaded.Version = fileFormatVersion
	loaded.DefaultPolicy = loaded.DefaultPolicy.Normalise()
	for i, l := range loaded.Licences {
		loaded.Licences[i] = l.Normalise()
	}
	sortLicences(loaded.Licences)
	s.data = loaded
	return s, nil
}

// snapshot is a copy of everything saved, which a reader can use without
// holding the lock.
func (s *store) snapshot() fileData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.data
	out.Licences = append([]eo.Licence(nil), s.data.Licences...)
	return out
}

// defaultPolicy returns the figures a new licence starts with.
func (s *store) defaultPolicy() eo.Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.DefaultPolicy
}

// change applies a change and saves the file. If saving fails the change is
// rolled back, so what is in memory always matches what is on disk.
func (s *store) change(apply func(d *fileData) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	beforeLicences := append([]eo.Licence(nil), s.data.Licences...)
	beforePolicy := s.data.DefaultPolicy

	if err := apply(&s.data); err != nil {
		s.data.Licences, s.data.DefaultPolicy = beforeLicences, beforePolicy
		return err
	}
	sortLicences(s.data.Licences)
	if err := s.save(); err != nil {
		s.data.Licences, s.data.DefaultPolicy = beforeLicences, beforePolicy
		return err
	}
	return nil
}

// save writes everything to the data file atomically: a temporary file in the
// same directory, flushed to disk, then renamed over the old one. A crash
// therefore leaves either the old file or the new one, never half of either.
func (s *store) save() error {
	s.data.Version = fileFormatVersion

	body, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("preparing the file: %w", err)
	}
	body = append(body, '\n')

	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename has succeeded

	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replacing %s: %w", s.path, err)
	}
	return nil
}

// sortLicences keeps the file in a stable order: the authorisation issued most
// recently first, and within a day by number, so the file and every list read
// the same way every time.
func sortLicences(all []eo.Licence) {
	sort.SliceStable(all, func(i, j int) bool {
		if !all[i].IssuedOn.Equal(all[j].IssuedOn) {
			return all[i].IssuedOn.After(all[j].IssuedOn)
		}
		return all[i].Label() < all[j].Label()
	})
}

// findLicence returns the position of a licence by identifier, or -1.
func findLicence(all []eo.Licence, id string) int {
	id = strings.TrimSpace(id)
	if id == "" {
		return -1
	}
	for i, l := range all {
		if l.ID == id {
			return i
		}
	}
	return -1
}

// nextID gives a new licence an identifier the user can read: its authorisation
// number where there is one, made unique.
func nextID(all []eo.Licence, l eo.Licence) string {
	base := idSafe(l.AuthorisationNo)
	if base == "" {
		base = idSafe(l.Label())
	}
	if base == "" {
		base = "licence"
	}
	id := base
	for n := 2; findLicence(all, id) >= 0; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

// idSafe reduces a reference to letters, digits and dashes, so that an
// identifier is safe in a URL and readable in the file. It is capped in total
// length, not only in the number of separators: an authorisation number pasted
// in with a page of text after it must not become an identifier a page of text
// long.
func idSafe(s string) string {
	const maxLen = 48
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(s) {
		if b.Len() >= maxLen {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// saveLicence adds a licence or replaces one that is already there, and returns
// it as saved.
func (s *store) saveLicence(l eo.Licence) (eo.Licence, error) {
	l = l.Normalise()
	if err := l.Validate(); err != nil {
		return l, err
	}
	err := s.change(func(d *fileData) error {
		if i := findLicence(d.Licences, l.ID); i >= 0 {
			l.ID = d.Licences[i].ID
			d.Licences[i] = l
			return nil
		}
		if len(d.Licences) >= maxLicences {
			return fmt.Errorf("there are already %d licences saved, which is the limit — remove one first", maxLicences)
		}
		l.ID = nextID(d.Licences, l)
		d.Licences = append(d.Licences, l)
		return nil
	})
	return l, err
}

// deleteLicence removes a licence.
func (s *store) deleteLicence(id string) error {
	return s.change(func(d *fileData) error {
		i := findLicence(d.Licences, id)
		if i < 0 {
			return fmt.Errorf("there is no licence %q saved", id)
		}
		d.Licences = append(d.Licences[:i:i], d.Licences[i+1:]...)
		return nil
	})
}

// getLicence returns one saved licence.
func (s *store) getLicence(id string) (eo.Licence, bool) {
	data := s.snapshot()
	if i := findLicence(data.Licences, id); i >= 0 {
		return data.Licences[i], true
	}
	return eo.Licence{}, false
}

// putDefaultPolicy replaces the figures a new licence starts with. It does NOT
// touch a licence already saved, for the reason given on fileData.
func (s *store) putDefaultPolicy(next eo.Policy) (eo.Policy, error) {
	next = next.Normalise()
	err := s.change(func(d *fileData) error {
		d.DefaultPolicy = next
		return nil
	})
	return next, err
}
