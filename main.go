// Copyright © 2022 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

// Nic-config tunes all physical network interfaces in a Linux machine for best performance.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/safchain/ethtool"
)

const (
	sysClassNet = "/sys/class/net"

	usage = `Usage: nic-config [-h] [-version] [-n]

Raises the rx/tx ring parameters of all physical network interfaces to their maximum values.

  -h        show this help
  -version  show the version
  -n        show what would change, without changing it
`
)

// ringDevice is the part of ethtool that nic-config uses, so that tests can replace real NICs.
type ringDevice interface {
	GetRing(intf string) (ethtool.Ring, error)
	SetRing(intf string, ring ethtool.Ring) (ethtool.Ring, error)
	Close()
}

// dryRunDevice reads from the wrapped device, but never changes it.
type dryRunDevice struct {
	ringDevice
}

// SetRing pretends that the driver applied the request as is.
func (dryRunDevice) SetRing(_ string, ring ethtool.Ring) (ethtool.Ring, error) {
	return ring, nil
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
	if errors.Is(err, syscall.EOPNOTSUPP) {
		// The driver has no ring parameters to tune.
		return nil
	}
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

// tune tunes every physical interface listed under root, carrying on after errors. It returns the exit status, 1 if
// anything failed.
func tune(root string, dev ringDevice, logger *log.Logger) int {
	devices, err := os.ReadDir(root)
	if err != nil {
		logger.Print(err)
		return 1
	}

	code := 0

	for _, link := range devices {
		intf := link.Name()

		physical, err := isPhysical(filepath.Join(root, intf))
		if err != nil {
			logger.Print(err)
			code = 1
			continue
		}
		if !physical {
			continue
		}

		if err := updateInterface(dev, intf, logger); err != nil {
			logger.Printf("%s: %v", intf, err)
			code = 1
		}
	}

	return code
}

// run handles the command line args, and only opens the device if there is tuning to do. It returns the exit status.
func run(args []string, root string, open func() (ringDevice, error), stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("nic-config", flag.ContinueOnError)
	flags.SetOutput(io.Discard) // messages are printed below
	version := flags.Bool("version", false, "")
	dryRun := flags.Bool("n", false, "")

	err := flags.Parse(args)
	if err == nil && flags.NArg() > 0 {
		err = fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}

	switch {
	case errors.Is(err, flag.ErrHelp):
		_, _ = fmt.Fprint(stdout, usage)

		return 0
	case err != nil:
		_, _ = fmt.Fprintf(stderr, "nic-config: %v\n%s", err, usage)

		return 2
	case *version:
		_, _ = fmt.Fprintf(stdout, "nic-config %s\n", Version)

		return 0
	}

	logger := log.New(stderr, "", 0)

	dev, err := open()
	if err != nil {
		logger.Print(err)
		return 1
	}
	defer dev.Close()

	if *dryRun {
		logger.SetPrefix("dry run: ")
		dev = dryRunDevice{dev}
	}

	return tune(root, dev, logger)
}

func main() {
	open := func() (ringDevice, error) { return ethtool.NewEthtool() }

	os.Exit(run(os.Args[1:], sysClassNet, open, os.Stdout, os.Stderr))
}
