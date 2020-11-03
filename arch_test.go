package main

import (
	"io/ioutil"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/cosandr/go-check-updates/api"
)

func TestArchParsePacmanCheckUpdates(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cache = NewInternalCache()
	out := `
cuda 11.1.0-2 -> 11.1.1-1
haskell-aeson 1.5.4.1-9 -> 1.5.4.1-10
haskell-assoc 1.0.2-22 -> 1.0.2-23
haskell-attoparsec 0.13.2.4-30 -> 0.13.2.4-31
haskell-base-compat-batteries 0.11.2-6 -> 0.11.2-7
lib32-vulkan-icd-loader 1.2.153-2 -> 1.2.158-1
libinput 1.16.2-1 -> 1.16.3-1
python-kiwisolver 1.3.0-1 -> 1.3.1-1
shellcheck 0.7.1-167 -> 0.7.1-168
vulkan-icd-loader 1.2.153-2 -> 1.2.158-1
`
	actual := parsePacmanCheckUpdates(out, rePacman, "pacman")
	expected := []api.Update{
		{
			Pkg:    "cuda",
			OldVer: "11.1.0-2",
			NewVer: "11.1.1-1",
			Repo:   "pacman",
		},
		{
			Pkg:    "haskell-aeson",
			OldVer: "1.5.4.1-9",
			NewVer: "1.5.4.1-10",
			Repo:   "pacman",
		},
		{
			Pkg:    "haskell-assoc",
			OldVer: "1.0.2-22",
			NewVer: "1.0.2-23",
			Repo:   "pacman",
		},
		{
			Pkg:    "haskell-attoparsec",
			OldVer: "0.13.2.4-30",
			NewVer: "0.13.2.4-31",
			Repo:   "pacman",
		},
		{
			Pkg:    "haskell-base-compat-batteries",
			OldVer: "0.11.2-6",
			NewVer: "0.11.2-7",
			Repo:   "pacman",
		},
		{
			Pkg:    "lib32-vulkan-icd-loader",
			OldVer: "1.2.153-2",
			NewVer: "1.2.158-1",
			Repo:   "pacman",
		},
		{
			Pkg:    "libinput",
			OldVer: "1.16.2-1",
			NewVer: "1.16.3-1",
			Repo:   "pacman",
		},
		{
			Pkg:    "python-kiwisolver",
			OldVer: "1.3.0-1",
			NewVer: "1.3.1-1",
			Repo:   "pacman",
		},
		{
			Pkg:    "shellcheck",
			OldVer: "0.7.1-167",
			NewVer: "0.7.1-168",
			Repo:   "pacman",
		},
		{
			Pkg:    "vulkan-icd-loader",
			OldVer: "1.2.153-2",
			NewVer: "1.2.158-1",
			Repo:   "pacman",
		},
	}
	if len(actual) != len(expected) {
		t.Errorf("expected %d updates, got %d", len(expected), len(actual))
	}
	for i, a := range actual {
		e := expected[i]
		if a.Pkg != e.Pkg {
			t.Errorf("expected name %s, got %s", e.Pkg, a.Pkg)
		}
		if a.OldVer != e.OldVer {
			t.Errorf("expected old version %s, got %s", e.OldVer, a.OldVer)
		}
		if a.NewVer != e.NewVer {
			t.Errorf("expected new version %s, got %s", e.NewVer, a.NewVer)
		}
		if a.Repo != e.Repo {
			t.Errorf("expected repo %s, got %s", e.Repo, a.Repo)
		}
	}
}

func TestArchPacmanLogs(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cache = NewInternalCache()
	cache.f.Checked = "2020-05-29T23:00:00+02:00"
	/*
		Check "new" log format (after 2019-10-24)
		Before that, the format was as below
		[2019-09-04 20:59] [ALPM] upgraded libgcrypt (1.8.4-1 -> 1.8.5-1)
		However, I don't see a point in parsing this, as the point of this program
		is to check for new updates and I don't expect anyone to not have upgraded
		for over a year
	*/
	file := `
[2020-05-29T23:47:18+0200] [ALPM] upgraded shellcheck (0.7.1-32 -> 0.7.1-33)
[2020-05-29T23:47:18+0200] [ALPM] upgraded vte-common (0.60.2-2 -> 0.60.3-1)
[2020-05-29T23:47:18+0200] [ALPM] upgraded vte3 (0.60.2-2 -> 0.60.3-1)
`
	allUpdates := []api.Update{
		{
			Pkg:    "shellcheck",
			NewVer: "0.7.1-33",
		},
		{
			Pkg:    "vte-common",
			NewVer: "0.60.3-1",
		},
		{
			Pkg:    "vte3",
			NewVer: "0.60.3-1",
		},
	}
	cache.f.Updates = allUpdates
	if err := runPacmanLogsTest(file); err != nil {
		t.Fatal(err)
	}
	if len(cache.f.Updates) > 0 {
		t.Errorf("Expected 0 updates, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
	// Add package that wasn't upgraded
	cache.f.Updates = append(allUpdates,
		api.Update{
			Pkg:    "keep-me",
			NewVer: "1.0",
		},
	)
	if err := runPacmanLogsTest(file); err != nil {
		t.Fatal(err)
	}
	if len(cache.f.Updates) != 1 {
		t.Errorf("Expected 1 update, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
	// Remove a package
	cache.f.Updates = allUpdates[1:]
	if err := runPacmanLogsTest(file); err != nil {
		t.Fatal(err)
	}
	if len(cache.f.Updates) != 0 {
		t.Errorf("Expected 0 updates, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
	// Update one with wrong version
	file = `
[2020-05-29T23:47:18+0200] [ALPM] upgraded shellcheck (0.7.1-32 -> 0.7.1-33)
[2020-05-29T23:47:18+0200] [ALPM] upgraded vte-common (0.60.2-2 -> 0.60.3-1)
[2020-05-29T23:47:18+0200] [ALPM] upgraded vte3 (0.60.2-2 -> 0.60.3)
`
	cache.f.Updates = allUpdates
	if err := runPacmanLogsTest(file); err != nil {
		t.Fatal(err)
	}
	if len(cache.f.Updates) != 1 {
		t.Errorf("Expected 1 update, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
	// Uninstall a package pending upgrades, note old version
	file = `
[2020-05-29T23:47:18+0200] [ALPM] upgraded shellcheck (0.7.1-32 -> 0.7.1-33)
[2020-05-29T23:47:18+0200] [ALPM] upgraded vte-common (0.60.2-2 -> 0.60.3-1)
[2020-05-29T23:47:18+0200] [ALPM] removed vte3 (0.60.2-2)
`
	cache.f.Updates = allUpdates
	if err := runPacmanLogsTest(file); err != nil {
		t.Fatal(err)
	}
	if len(cache.f.Updates) != 0 {
		t.Errorf("Expected 0 updates, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
}

func runPacmanLogsTest(content string) error {
	fp := "/tmp/arch_test.log"
	err := ioutil.WriteFile(fp, []byte(content), 0644)
	if err != nil {
		return err
	}
	err = checkPacmanLogs(fp)
	if err != nil {
		return err
	}
	return nil
}
