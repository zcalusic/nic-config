// Copyright © 2022 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

// Nic-config tunes all physical network interfaces in a Linux machine for best performance.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/safchain/ethtool"
)

const (
	sysClassNet = "/sys/class/net"
)

// ringDevice is the part of ethtool that nic-config uses, so that tests can replace real NICs.
type ringDevice interface {
	GetRing(intf string) (ethtool.Ring, error)
	SetRing(intf string, ring ethtool.Ring) (ethtool.Ring, error)
}

// maximizeRing raises the rx/tx ring sizes to their maximums, and reports whether that changed anything.
func maximizeRing(ring ethtool.Ring) (ethtool.Ring, bool) {
	changed := ring.RxPending < ring.RxMaxPending || ring.TxPending < ring.TxMaxPending

	ring.RxPending = max(ring.RxPending, ring.RxMaxPending)
	ring.TxPending = max(ring.TxPending, ring.TxMaxPending)

	return ring, changed
}

func updateInterface(dev ringDevice, intf string, logger *log.Logger) error {
	ring, err := dev.GetRing(intf)
	if err != nil {
		return err
	}

	maximized, changed := maximizeRing(ring)
	if !changed {
		return nil
	}

	applied, err := dev.SetRing(intf, maximized)
	if err != nil {
		return err
	}

	logger.Printf("%s: rx %d -> %d | tx %d -> %d", intf, ring.RxPending, applied.RxPending, ring.TxPending, applied.TxPending)

	return nil
}

// isPhysical reports whether the entry at fullpath, under /sys/class/net, is a real network device.
func isPhysical(fullpath string) (bool, error) {
	fi, err := os.Lstat(fullpath)
	if err != nil {
		return false, err
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return false, nil
	}

	dev, err := os.Readlink(fullpath)
	if err != nil {
		return false, err
	}

	// Skip virtual devices.
	return !strings.HasPrefix(dev, "../../devices/virtual/"), nil
}

// run tunes every physical interface listed under root, and returns the exit status.
func run(root string, dev ringDevice, logger *log.Logger) int {
	devices, err := os.ReadDir(root)
	if err != nil {
		logger.Print(err)
		return 1
	}

	for _, link := range devices {
		intf := link.Name()

		physical, err := isPhysical(filepath.Join(root, intf))
		if err != nil {
			logger.Print(err)
			continue
		}
		if !physical {
			continue
		}

		if err := updateInterface(dev, intf, logger); err != nil {
			logger.Printf("%s: %v", intf, err)
		}
	}

	return 0
}

func main() {
	logger := log.New(os.Stderr, "", 0)

	eth, err := ethtool.NewEthtool()
	if err != nil {
		logger.Print(err)
		return
	}

	code := run(sysClassNet, eth, logger)
	eth.Close()
	os.Exit(code)
}
