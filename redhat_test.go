package main

import (
	"io/ioutil"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/cosandr/go-check-updates/api"
)

func TestDnfLogs(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cache = InternalCache{
		f: api.File{
			Checked: "2020-07-12T11:00:00+02:00",
		},
	}
	// Check "old" log format (date unknown)
	file := `
2020-07-12T11:13:51Z SUBDEBUG Upgrade: selinux-policy-devel-3.14.5-42.fc32.noarch
2020-07-12T11:14:13Z SUBDEBUG Upgrade: x264-libs-0.159-10.20200409git296494a.fc32.x86_64
2020-07-12T11:14:13Z SUBDEBUG Upgrade: grafana-7.0.6-1.x86_64
`
	allUpdates := []api.Update{
		{
			Pkg:    "selinux-policy-devel",
			NewVer: "3.14.5-42",
		},
		{
			Pkg:    "x264-libs",
			NewVer: "0.159-10.20200409git296494a",
		},
		{
			Pkg:    "grafana",
			NewVer: "7.0.6-1",
		},
	}
	cache.f.Updates = allUpdates
	if err := runDnfLogsTest(file); err != nil {
		t.Fatal(err)
	}
	if len(cache.f.Updates) != 0 {
		t.Errorf("Expected 0 updates, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
	// Update one with wrong version
	file = `
2020-07-12T11:13:51Z SUBDEBUG Upgrade: selinux-policy-devel-3.14.5-42.fc32.noarch
2020-07-12T11:14:13Z SUBDEBUG Upgrade: x264-libs-0.159-10.20200409git296494a.fc32.x86_64
2020-07-12T11:14:13Z SUBDEBUG Upgrade: grafana-7.0.6.x86_64
`
	cache.f.Updates = allUpdates
	if err := runDnfLogsTest(file); err != nil {
		t.Fatal(err)
	}
	if len(cache.f.Updates) != 1 {
		t.Errorf("Expected 1 update, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
	// Uninstall a package pending upgrades, note old version
	file = `
2020-07-12T11:13:51Z SUBDEBUG Upgrade: selinux-policy-devel-3.14.5-42.fc32.noarch
2020-07-12T11:14:13Z SUBDEBUG Upgrade: x264-libs-0.159-10.20200409git296494a.fc32.x86_64
2020-07-12T11:14:13Z SUBDEBUG Erase: grafana-7.0.5.x86_64
`
	cache.f.Updates = allUpdates
	if err := runDnfLogsTest(file); err != nil {
		t.Fatal(err)
	}
	if len(cache.f.Updates) != 0 {
		t.Errorf("Expected 0 updates, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
	// New format
	file = `
2020-11-01T12:10:42+0100 SUBDEBUG Upgrade: xen-libs-4.13.1-7.fc32.x86_64
2020-11-01T12:10:42+0100 SUBDEBUG Upgrade: grafana-7.3.1-1.x86_64
2020-11-01T12:10:46+0100 SUBDEBUG Upgrade: perl-Digest-1.19-1.fc32.noarch
`
	cache.f.Updates = []api.Update{
		{
			Pkg:    "xen-libs",
			NewVer: "4.13.1-7",
		},
		{
			Pkg:    "grafana",
			NewVer: "7.3.1-1",
		},
		{
			Pkg:    "perl-Digest",
			NewVer: "1.19-1",
		},
	}
	if err := runDnfLogsTest(file); err != nil {
		t.Fatal(err)
	}
	if len(cache.f.Updates) != 0 {
		t.Errorf("Expected 0 updates, got %d: %v", len(cache.f.Updates), cache.f.Updates)
	}
}

func runDnfLogsTest(content string) error {
	fp := "/tmp/dnf_test.log"
	err := ioutil.WriteFile(fp, []byte(content), 0644)
	if err != nil {
		return err
	}
	err = checkDnfLogs(fp)
	if err != nil {
		return err
	}
	return nil
}
