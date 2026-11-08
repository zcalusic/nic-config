// Copyright © 2026 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/safchain/ethtool"
)

var errNotSupported = errors.New("operation not supported")

type setCall struct {
	intf string
	ring ethtool.Ring
}

// fakeDevice stands in for ethtool, and records the calls it receives.
type fakeDevice struct {
	rings   map[string]ethtool.Ring
	applied map[string]ethtool.Ring // what SetRing returns, when the driver doesn't apply the request as is
	getErr  map[string]error
	setErr  map[string]error

	gets []string
	sets []setCall
}

func (f *fakeDevice) GetRing(intf string) (ethtool.Ring, error) {
	f.gets = append(f.gets, intf)
	if err := f.getErr[intf]; err != nil {
		return ethtool.Ring{}, err
	}
	return f.rings[intf], nil
}

func (f *fakeDevice) SetRing(intf string, ring ethtool.Ring) (ethtool.Ring, error) {
	f.sets = append(f.sets, setCall{intf, ring})
	if err := f.setErr[intf]; err != nil {
		return ethtool.Ring{}, err
	}
	if applied, ok := f.applied[intf]; ok {
		return applied, nil
	}
	return ring, nil
}

func newRing(rx, rxMax, tx, txMax uint32) ethtool.Ring {
	return ethtool.Ring{RxPending: rx, RxMaxPending: rxMax, TxPending: tx, TxMaxPending: txMax}
}

// makeTree builds a fixture of /sys/class/net, with links laid out the same way as in sysfs.
func makeTree(t *testing.T, physical, virtual, files []string) string {
	t.Helper()

	root := t.TempDir()
	for _, name := range physical {
		if err := os.Symlink("../../devices/pci0000:00/0000:00:1f.6/net/"+name, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range virtual {
		if err := os.Symlink("../../devices/virtual/net/"+name, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

func TestMaximizeRing(t *testing.T) {
	tests := []struct {
		name        string
		ring        ethtool.Ring
		want        ethtool.Ring
		wantChanged bool
	}{
		{"both below max", newRing(256, 4096, 256, 4096), newRing(4096, 4096, 4096, 4096), true},
		{"only rx below max", newRing(256, 4096, 1024, 1024), newRing(4096, 4096, 1024, 1024), true},
		{"only tx below max", newRing(1024, 1024, 256, 4096), newRing(1024, 1024, 4096, 4096), true},
		{"already at max", newRing(4096, 4096, 4096, 4096), newRing(4096, 4096, 4096, 4096), false},
		{"no maximums reported", newRing(0, 0, 0, 0), newRing(0, 0, 0, 0), false},
		{
			"mini and jumbo left alone",
			ethtool.Ring{RxPending: 1, RxMaxPending: 2, RxMiniPending: 3, RxMiniMaxPending: 4, RxJumboPending: 5, RxJumboMaxPending: 6},
			ethtool.Ring{RxPending: 2, RxMaxPending: 2, RxMiniPending: 3, RxMiniMaxPending: 4, RxJumboPending: 5, RxJumboMaxPending: 6},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := maximizeRing(tt.ring)
			if got != tt.want {
				t.Errorf("ring = %+v, want %+v", got, tt.want)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
		})
	}
}

func TestRun(t *testing.T) {
	maximized := newRing(4096, 4096, 4096, 4096)

	tests := []struct {
		name     string
		physical []string
		virtual  []string
		files    []string
		dev      fakeDevice
		wantGets []string
		wantSets []setCall
		wantLog  string
	}{
		{
			name:     "both below max",
			physical: []string{"eth0"},
			dev:      fakeDevice{rings: map[string]ethtool.Ring{"eth0": newRing(256, 4096, 256, 4096)}},
			wantGets: []string{"eth0"},
			wantSets: []setCall{{"eth0", maximized}},
			wantLog:  "eth0: rx 256 -> 4096 | tx 256 -> 4096\n",
		},
		{
			name:     "only rx below max",
			physical: []string{"eth0"},
			dev:      fakeDevice{rings: map[string]ethtool.Ring{"eth0": newRing(256, 4096, 1024, 1024)}},
			wantGets: []string{"eth0"},
			wantSets: []setCall{{"eth0", newRing(4096, 4096, 1024, 1024)}},
			wantLog:  "eth0: rx 256 -> 4096 | tx 1024 -> 1024\n",
		},
		{
			name:     "only tx below max",
			physical: []string{"eth0"},
			dev:      fakeDevice{rings: map[string]ethtool.Ring{"eth0": newRing(1024, 1024, 256, 4096)}},
			wantGets: []string{"eth0"},
			wantSets: []setCall{{"eth0", newRing(1024, 1024, 4096, 4096)}},
			wantLog:  "eth0: rx 1024 -> 1024 | tx 256 -> 4096\n",
		},
		{
			name:     "already at max",
			physical: []string{"eth0"},
			dev:      fakeDevice{rings: map[string]ethtool.Ring{"eth0": maximized}},
			wantGets: []string{"eth0"},
		},
		{
			name:     "no maximums reported",
			physical: []string{"eth0"},
			dev:      fakeDevice{rings: map[string]ethtool.Ring{"eth0": {}}},
			wantGets: []string{"eth0"},
		},
		{
			name:     "log shows what the driver applied",
			physical: []string{"eth0"},
			dev: fakeDevice{
				rings:   map[string]ethtool.Ring{"eth0": newRing(256, 4096, 256, 4096)},
				applied: map[string]ethtool.Ring{"eth0": newRing(2048, 4096, 1024, 4096)},
			},
			wantGets: []string{"eth0"},
			wantSets: []setCall{{"eth0", maximized}},
			wantLog:  "eth0: rx 256 -> 2048 | tx 256 -> 1024\n",
		},
		{
			name:    "virtual device skipped",
			virtual: []string{"lo"},
		},
		{
			name:  "regular file skipped",
			files: []string{"bonding_masters"},
		},
		{
			name: "empty tree",
		},
		{
			name:     "only physical devices, in name order",
			physical: []string{"eth1", "eth0"},
			virtual:  []string{"br0", "lo"},
			files:    []string{"bonding_masters"},
			dev: fakeDevice{rings: map[string]ethtool.Ring{
				"eth0": newRing(256, 4096, 256, 4096),
				"eth1": newRing(512, 4096, 512, 4096),
			}},
			wantGets: []string{"eth0", "eth1"},
			wantSets: []setCall{{"eth0", maximized}, {"eth1", maximized}},
			wantLog:  "eth0: rx 256 -> 4096 | tx 256 -> 4096\neth1: rx 512 -> 4096 | tx 512 -> 4096\n",
		},
		{
			name:     "GetRing error, next interface still tuned",
			physical: []string{"eth0", "eth1"},
			dev: fakeDevice{
				rings:  map[string]ethtool.Ring{"eth1": newRing(256, 4096, 256, 4096)},
				getErr: map[string]error{"eth0": errNotSupported},
			},
			wantGets: []string{"eth0", "eth1"},
			wantSets: []setCall{{"eth1", maximized}},
			wantLog:  "eth0: operation not supported\neth1: rx 256 -> 4096 | tx 256 -> 4096\n",
		},
		{
			name:     "SetRing error, next interface still tuned",
			physical: []string{"eth0", "eth1"},
			dev: fakeDevice{
				rings: map[string]ethtool.Ring{
					"eth0": newRing(256, 4096, 256, 4096),
					"eth1": newRing(256, 4096, 256, 4096),
				},
				setErr: map[string]error{"eth0": errNotSupported},
			},
			wantGets: []string{"eth0", "eth1"},
			wantSets: []setCall{{"eth0", maximized}, {"eth1", maximized}},
			wantLog:  "eth0: operation not supported\neth1: rx 256 -> 4096 | tx 256 -> 4096\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := makeTree(t, tt.physical, tt.virtual, tt.files)

			var buf bytes.Buffer
			if code := run(root, &tt.dev, log.New(&buf, "", 0)); code != 0 {
				t.Errorf("exit status = %d, want 0", code)
			}

			if !slices.Equal(tt.dev.gets, tt.wantGets) {
				t.Errorf("GetRing calls = %v, want %v", tt.dev.gets, tt.wantGets)
			}
			if !slices.Equal(tt.dev.sets, tt.wantSets) {
				t.Errorf("SetRing calls = %+v, want %+v", tt.dev.sets, tt.wantSets)
			}
			if got := buf.String(); got != tt.wantLog {
				t.Errorf("log = %q, want %q", got, tt.wantLog)
			}
		})
	}
}

func TestRunMissingRoot(t *testing.T) {
	var dev fakeDevice
	var buf bytes.Buffer

	root := filepath.Join(t.TempDir(), "missing")
	if code := run(root, &dev, log.New(&buf, "", 0)); code != 1 {
		t.Errorf("exit status = %d, want 1", code)
	}
	if len(dev.gets) != 0 {
		t.Errorf("GetRing calls = %v, want none", dev.gets)
	}
	if got := buf.String(); !strings.Contains(got, root) {
		t.Errorf("log = %q, want it to name %q", got, root)
	}
}

func TestIsPhysicalMissing(t *testing.T) {
	if _, err := isPhysical(filepath.Join(t.TempDir(), "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want os.ErrNotExist", err)
	}
}
