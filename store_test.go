package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sreenivas-sadhu-prabhakara/epcg-advance-authorisation-export-obligation/internal/eo"
)

func tempStore(t *testing.T) *store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "licences.json")
	s, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	return s
}

func minimalLicence(authNo string) eo.Licence {
	return eo.Licence{
		Scheme:             eo.SchemeEPCG,
		AuthorisationNo:    authNo,
		IssuedOn:           eo.MustDate("2024-04-01"),
		Category:           eo.CategoryAsEndorsed,
		SpecificEOEndorsed: eo.MustMoney("1,00,00,000"),
		AverageEOExempt:    true,
	}
}

func TestOpenStoreOnNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "none.json")
	s, err := openStore(path)
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	if got := len(s.snapshot().Licences); got != 0 {
		t.Fatalf("%d licences in an empty store", got)
	}
	// The file is not created until something is saved.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the data file should not exist yet")
	}
	if s.defaultPolicy().EPCGMultipleHundredths != 600 {
		t.Fatalf("a new store starts with the published figures")
	}
}

func TestSaveAndReloadALicence(t *testing.T) {
	s := tempStore(t)

	saved, err := s.saveLicence(minimalLicence("0330099887"))
	if err != nil {
		t.Fatalf("saveLicence: %v", err)
	}
	if saved.ID != "0330099887" {
		t.Fatalf("id = %q; it should be readable", saved.ID)
	}

	// The file exists, is readable only by its owner, and is plain JSON.
	info, err := os.Stat(s.path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("file mode = %v, want 0600", perm)
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var check fileData
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Fatalf("the saved file is not readable JSON: %v", err)
	}
	if check.Version != fileFormatVersion {
		t.Fatalf("version = %d", check.Version)
	}

	// Reopening gives the same licence back.
	reopened, err := openStore(s.path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, ok := reopened.getLicence("0330099887")
	if !ok {
		t.Fatalf("the licence did not survive a reload")
	}
	if got.SpecificEOEndorsed != eo.MustMoney("1,00,00,000") {
		t.Fatalf("obligation = %s", got.SpecificEOEndorsed)
	}
}

func TestSaveLicenceReplacesRatherThanDuplicating(t *testing.T) {
	s := tempStore(t)

	first, err := s.saveLicence(minimalLicence("0330099887"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	changed := first
	changed.Holder = "Nandi Precision Components"
	again, err := s.saveLicence(changed)
	if err != nil {
		t.Fatalf("resave: %v", err)
	}
	if again.ID != first.ID {
		t.Fatalf("the identifier changed on a save: %q then %q", first.ID, again.ID)
	}
	if got := len(s.snapshot().Licences); got != 1 {
		t.Fatalf("%d licences; saving the same one twice must not duplicate it", got)
	}
	back, _ := s.getLicence(first.ID)
	if back.Holder != "Nandi Precision Components" {
		t.Fatalf("the change was not saved: %q", back.Holder)
	}
}

func TestIdentifiersAreMadeUnique(t *testing.T) {
	s := tempStore(t)

	a, _ := s.saveLicence(minimalLicence("0330099887"))
	b, err := s.saveLicence(minimalLicence("0330099887"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if a.ID == b.ID {
		t.Fatalf("two licences share the identifier %q", a.ID)
	}
	if b.ID != "0330099887-2" {
		t.Fatalf("second id = %q, want 0330099887-2", b.ID)
	}

	// A licence with no authorisation number still gets a usable identifier.
	noNumber := minimalLicence("")
	noNumber.Product = "Machined housings"
	c, err := s.saveLicence(noNumber)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if c.ID != "machined-housings" {
		t.Fatalf("id = %q", c.ID)
	}
}

func TestIdSafe(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"0330099887", "0330099887"},
		{"03/21/000/00412/AM24", "03-21-000-00412-am24"},
		{"  Machined   Housings  ", "machined-housings"},
		{"!!!", ""},
		{"", ""},
		{strings.Repeat("ab ", 40), strings.TrimSuffix(strings.Repeat("ab-", 17), "-") + "ab"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := idSafe(c.in)
			if c.in == strings.Repeat("ab ", 40) {
				// Only the cap matters here, not the exact tail.
				if len(got) > 48 {
					t.Fatalf("identifier is %d characters: %q", len(got), got)
				}
				return
			}
			if got != c.want {
				t.Fatalf("idSafe(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestSaveLicenceRefusesAnInvalidOne(t *testing.T) {
	s := tempStore(t)
	bad := minimalLicence("BAD")
	bad.IssuedOn = eo.Date{}
	if _, err := s.saveLicence(bad); err == nil {
		t.Fatalf("a licence with no date of issue must be refused")
	}
	if got := len(s.snapshot().Licences); got != 0 {
		t.Fatalf("%d licences saved after a refusal", got)
	}
	// And no file was created by the failed attempt.
	if _, err := os.Stat(s.path); !os.IsNotExist(err) {
		t.Fatalf("a refused save must not create the file")
	}
}

func TestDeleteLicence(t *testing.T) {
	s := tempStore(t)
	if _, err := s.saveLicence(minimalLicence("A")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := s.saveLicence(minimalLicence("B")); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.deleteLicence("a"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := s.getLicence("a"); ok {
		t.Fatalf("the licence is still there")
	}
	if _, ok := s.getLicence("b"); !ok {
		t.Fatalf("the other licence went too")
	}
	if err := s.deleteLicence("a"); err == nil {
		t.Fatalf("deleting what is not there must report it")
	}
	if err := s.deleteLicence(""); err == nil {
		t.Fatalf("an empty identifier must be refused")
	}
}

func TestLicencesAreSortedNewestIssueFirst(t *testing.T) {
	s := tempStore(t)
	for _, c := range []struct {
		no     string
		issued string
	}{
		{"OLD", "2020-04-01"},
		{"NEW", "2026-04-01"},
		{"MID", "2023-04-01"},
	} {
		l := minimalLicence(c.no)
		l.IssuedOn = eo.MustDate(c.issued)
		if _, err := s.saveLicence(l); err != nil {
			t.Fatalf("save %s: %v", c.no, err)
		}
	}
	got := s.snapshot().Licences
	want := []string{"NEW", "MID", "OLD"}
	for i := range want {
		if got[i].AuthorisationNo != want[i] {
			t.Fatalf("order = %q at %d, want %q", got[i].AuthorisationNo, i, want[i])
		}
	}
}

func TestPutDefaultPolicyDoesNotReachIntoASavedLicence(t *testing.T) {
	// The whole reason the policy lives on the licence: an authorisation goes
	// on being governed by the figures that applied to it when it was issued.
	s := tempStore(t)
	saved, err := s.saveLicence(minimalLicence("KEEP"))
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.Policy.InterestRatePercent != 15 {
		t.Fatalf("rate = %d", saved.Policy.InterestRatePercent)
	}

	next := eo.AsPublished()
	next.InterestRatePercent = 18
	got, err := s.putDefaultPolicy(next)
	if err != nil {
		t.Fatalf("putDefaultPolicy: %v", err)
	}
	if got.InterestRatePercent != 18 || !got.Edited {
		t.Fatalf("default policy = %+v", got)
	}

	still, _ := s.getLicence("keep")
	if still.Policy.InterestRatePercent != 15 {
		t.Fatalf("the saved licence's rate changed to %d", still.Policy.InterestRatePercent)
	}

	// A new licence picks the new default up when it is given no policy of its
	// own, which the API does.
	if s.defaultPolicy().InterestRatePercent != 18 {
		t.Fatalf("the default did not stick")
	}
}

func TestOpenStoreRefusesWhatItCannotRead(t *testing.T) {
	dir := t.TempDir()

	t.Run("a file that is not JSON", func(t *testing.T) {
		path := filepath.Join(dir, "junk.json")
		if err := os.WriteFile(path, []byte("this is not JSON"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, err := openStore(path); err == nil {
			t.Fatalf("expected a refusal")
		} else if !strings.Contains(err.Error(), "move the file aside") {
			t.Fatalf("the message should say what to do: %v", err)
		}
	})

	t.Run("a file from a newer version", func(t *testing.T) {
		path := filepath.Join(dir, "future.json")
		if err := os.WriteFile(path, []byte(`{"version":99,"licences":[]}`), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		if _, err := openStore(path); err == nil {
			t.Fatalf("expected a refusal")
		}
	})

	t.Run("an empty file is an empty store, not an error", func(t *testing.T) {
		path := filepath.Join(dir, "empty.json")
		if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		s, err := openStore(path)
		if err != nil {
			t.Fatalf("openStore: %v", err)
		}
		if got := len(s.snapshot().Licences); got != 0 {
			t.Fatalf("%d licences", got)
		}
	})
}

func TestSaveIsAtomic(t *testing.T) {
	// A save writes a temporary file and renames it over the old one, so a
	// crash leaves either the old file or the new one. The observable part of
	// that contract: no temporary files are left behind.
	s := tempStore(t)
	for i := 0; i < 5; i++ {
		l := minimalLicence("A")
		l.Notes = strings.Repeat("x", i*100)
		if _, err := s.saveLicence(l); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(s.path))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Fatalf("a temporary file was left behind: %s", e.Name())
		}
	}
	if len(entries) != 1 {
		t.Fatalf("%d files in the directory, want just the data file", len(entries))
	}
}

func TestStoreHoldsALimitedNumberOfLicences(t *testing.T) {
	s := tempStore(t)
	s.mu.Lock()
	for i := 0; i < maxLicences; i++ {
		l := minimalLicence("L" + strings.Repeat("0", 3) + itoa(i))
		l.ID = l.AuthorisationNo
		s.data.Licences = append(s.data.Licences, l.Normalise())
	}
	s.mu.Unlock()

	if _, err := s.saveLicence(minimalLicence("ONEMORE")); err == nil {
		t.Fatalf("expected a refusal at the limit")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
