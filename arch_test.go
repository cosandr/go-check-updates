package main

import (
	"io/ioutil"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/cosandr/go-check-updates/api"
)

func TestPacmanLogs(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cache = InternalCache{
		f: api.File{
			Checked: "2020-05-29T23:00:00+02:00",
		},
	}
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
