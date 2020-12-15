package main

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	log "github.com/sirupsen/logrus"

	"github.com/cosandr/go-check-updates/api"
)

func TestRedHatParseYumCheckUpdate(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cache = NewInternalCache()
	out := `
efivar-libs.x86_64                                                        37-14.fc33                                             updates
elfutils.x86_64                                                           0.182-1.fc33                                           updates
elfutils-debuginfod-client.x86_64                                         0.182-1.fc33                                           updates
elfutils-default-yama-scope.noarch                                        0.182-1.fc33                                           updates
ima-evm-utils.x86_64                                                      1.3.2-1.fc33                                           updates
iputils.x86_64                                                            20200821-1.fc33                                        updates
kernel-core.x86_64                                                        5.8.17-300.fc33                                        updates
libsmbclient.x86_64                                                       2:4.13.1-0.fc33                                        updates
libwbclient.x86_64                                                        2:4.13.1-0.fc33                                        updates
pcre2.x86_64                                                              10.35-8.fc33                                           updates
pcre2-devel.x86_64                                                        10.35-8.fc33                                           updates
pcre2-syntax.noarch                                                       10.35-8.fc33                                           updates
samba.x86_64                                                              2:4.13.1-0.fc33                                        updates
samba-common.noarch                                                       2:4.13.1-0.fc33                                        updates
`
	actual := parseYumCheckUpdate(out)
	expected := api.UpdatesList{
		{
			Pkg:    "efivar-libs",
			NewVer: "37-14.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "elfutils",
			NewVer: "0.182-1.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "elfutils-debuginfod-client",
			NewVer: "0.182-1.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "elfutils-default-yama-scope",
			NewVer: "0.182-1.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "ima-evm-utils",
			NewVer: "1.3.2-1.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "iputils",
			NewVer: "20200821-1.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "kernel-core",
			NewVer: "5.8.17-300.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "libsmbclient",
			NewVer: "2:4.13.1-0.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "libwbclient",
			NewVer: "2:4.13.1-0.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "pcre2",
			NewVer: "10.35-8.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "pcre2-devel",
			NewVer: "10.35-8.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "pcre2-syntax",
			NewVer: "10.35-8.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "samba",
			NewVer: "2:4.13.1-0.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "samba-common",
			NewVer: "2:4.13.1-0.fc33",
			Repo:   "updates",
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
		if a.NewVer != e.NewVer {
			t.Errorf("expected version %s, got %s", e.NewVer, a.NewVer)
		}
		if a.Repo != e.Repo {
			t.Errorf("expected repo %s, got %s", e.Repo, a.Repo)
		}
	}
	// Test when Obsoleting Packages is present
	out = `
kernel-core.x86_64                                                   5.8.18-300.fc33                                             updates
kernel-devel.x86_64                                                  5.8.18-300.fc33                                             updates
kernel-headers.x86_64                                                5.8.18-300.fc33                                             updates
kernel-modules.x86_64                                                5.8.18-300.fc33                                             updates
Obsoleting Packages
kernel-headers.x86_64                                                5.8.18-300.fc33                                             updates
    kernel-headers.x86_64                                            5.8.11-300.fc33                                             @fedora
`
	actual = parseYumCheckUpdate(out)
	expected = api.UpdatesList{
		{
			Pkg:    "kernel-core",
			NewVer: "5.8.18-300.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "kernel-devel",
			NewVer: "5.8.18-300.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "kernel-headers",
			NewVer: "5.8.18-300.fc33",
			Repo:   "updates",
		},
		{
			Pkg:    "kernel-modules",
			NewVer: "5.8.18-300.fc33",
			Repo:   "updates",
		},
	}
	if len(actual) != len(expected) {
		t.Fatalf("expected %d updates, got %d", len(expected), len(actual))
	}
	for i, a := range actual {
		e := expected[i]
		if a.Pkg != e.Pkg {
			t.Errorf("expected name %s, got %s", e.Pkg, a.Pkg)
		}
		if a.NewVer != e.NewVer {
			t.Errorf("expected version %s, got %s", e.NewVer, a.NewVer)
		}
		if a.Repo != e.Repo {
			t.Errorf("expected repo %s, got %s", e.Repo, a.Repo)
		}
	}
}

func TestRedHatDnfLogs(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	cache = NewInternalCache()
	cache.f.Checked = "2020-07-12T11:00:00+02:00"
	// Check "old" log format (date unknown)
	file := `
2020-07-12T11:13:51Z SUBDEBUG Upgrade: selinux-policy-devel-3.14.5-42.fc32.noarch
2020-07-12T11:14:13Z SUBDEBUG Upgrade: x264-libs-0.159-10.20200409git296494a.fc32.x86_64
2020-07-12T11:14:13Z SUBDEBUG Upgrade: grafana-7.0.6-1.x86_64
`
	allUpdates := api.UpdatesList{
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
	cache.f.Updates = api.UpdatesList{
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
	// Test on real logs
	cache = NewInternalCache()
	file = `{"checked":"2020-11-02T23:20:07+01:00","updates":[{"pkg":"binutils-gold","newVer":"2.35-14.fc33","repo":"updates"},{"pkg":"binutils","newVer":"2.35-14.fc33","repo":"updates"},{"pkg":"gstreamer1-plugins-bad-free","newVer":"1.18.1-1.fc33","repo":"updates"},{"pkg":"gstreamer1-plugins-base","newVer":"1.18.1-1.fc33","repo":"updates"},{"pkg":"gstreamer1","newVer":"1.18.1-1.fc33","repo":"updates"},{"pkg":"intel-mediasdk","newVer":"20.3.1-1.fc33","repo":"updates"},{"pkg":"lmdb-libs","newVer":"0.9.27-1.fc33","repo":"updates"},{"pkg":"pam","newVer":"1.4.0-6.fc33","repo":"updates"}]}`
	err := json.Unmarshal([]byte(file), &cache.f)
	if err != nil {
		t.Fatal(err)
	}
	file = `
2020-11-02T23:44:37+0100 SUBDEBUG Upgrade: gstreamer1-1.18.1-1.fc33.x86_64
2020-11-02T23:44:37+0100 SUBDEBUG Upgrade: gstreamer1-plugins-base-1.18.1-1.fc33.x86_64
2020-11-02T23:44:37+0100 SUBDEBUG Upgrade: binutils-gold-2.35-14.fc33.x86_64
2020-11-02T23:44:37+0100 SUBDEBUG Upgrade: binutils-2.35-14.fc33.x86_64
2020-11-02T23:44:37+0100 SUBDEBUG Upgrade: gstreamer1-plugins-bad-free-1.18.1-1.fc33.x86_64
2020-11-02T23:44:38+0100 SUBDEBUG Upgrade: pam-1.4.0-6.fc33.x86_64
2020-11-02T23:44:38+0100 SUBDEBUG Upgrade: lmdb-libs-0.9.27-1.fc33.x86_64
2020-11-02T23:44:38+0100 SUBDEBUG Upgrade: intel-mediasdk-20.3.1-1.fc33.x86_64
2020-11-02T23:44:38+0100 SUBDEBUG Upgraded: gstreamer1-plugins-bad-free-1.18.0-3.fc33.x86_64
2020-11-02T23:44:38+0100 SUBDEBUG Upgraded: gstreamer1-plugins-base-1.18.0-1.fc33.x86_64
2020-11-02T23:44:38+0100 SUBDEBUG Upgraded: binutils-2.35-11.fc33.x86_64
2020-11-02T23:44:38+0100 SUBDEBUG Upgraded: binutils-gold-2.35-11.fc33.x86_64
2020-11-02T23:44:39+0100 SUBDEBUG Upgraded: gstreamer1-1.18.0-1.fc33.x86_64
2020-11-02T23:44:39+0100 SUBDEBUG Upgraded: pam-1.4.0-5.fc33.x86_64
2020-11-02T23:44:39+0100 SUBDEBUG Upgraded: lmdb-libs-0.9.26-1.fc33.x86_64
2020-11-02T23:44:39+0100 SUBDEBUG Upgraded: intel-mediasdk-20.3.0-1.fc33.x86_64
`
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
