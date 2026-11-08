// Copyright © 2022 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

// Nic-config tunes all physical network interfaces in a Linux machine for best performance.
package main

import (
	"log"
	"os"
	"path"
	"strings"

	"github.com/safchain/ethtool"
)

const (
	sysClassNet = "/sys/class/net"
)

func updateInterface(intf string) error {
	eth, err := ethtool.NewEthtool()
	if err != nil {
		return err
	}
	defer eth.Close()

	ring, err := eth.GetRing(intf)
	if err != nil {
		return err
	}

	rx := ring.RxPending
	tx := ring.TxPending

	var needUpdate bool

	if ring.RxPending < ring.RxMaxPending {
		ring.RxPending = ring.RxMaxPending
		needUpdate = true
	}

	if ring.TxPending < ring.TxMaxPending {
		ring.TxPending = ring.TxMaxPending
		needUpdate = true
	}

	if needUpdate {
		ring, err = eth.SetRing(intf, ring)
		if err != nil {
			return err
		}
		log.Printf("%s: rx %d -> %d | tx %d -> %d", intf, rx, ring.RxPending, tx, ring.TxPending)
	}

	return nil
}

func main() {
	log.SetFlags(0)

	devices, err := os.ReadDir(sysClassNet)
	if err != nil {
		log.Fatal(err)
	}

	for _, link := range devices {
		intf := link.Name()
		fullpath := path.Join(sysClassNet, intf)
		fi, err := os.Lstat(fullpath)
		if err != nil {
			log.Print(err)
			continue
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		dev, err := os.Readlink(fullpath)
		if err != nil {
			log.Print(err)
			continue
		}

		// Skip virtual devices.
		if strings.HasPrefix(dev, "../../devices/virtual/") {
			continue
		}

		if err := updateInterface(intf); err != nil {
			log.Print(err)
			continue
		}
	}
}
