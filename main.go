// Copyright © 2022 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"log"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const (
	sysClassNet = "/sys/class/net"
	ethTool     = "ethtool"
)

var (
	reRX = regexp.MustCompile(`^RX:\s+(\d+)$`)
	reTX = regexp.MustCompile(`^TX:\s+(\d+)$`)
)

func prepareCmd(app string, args []string) *exec.Cmd {
	log.Printf("Running %s %s", app, strings.Join(args, " "))

	return exec.Command(app, args...)
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

		var stdout, stderr bytes.Buffer

		cmd := prepareCmd(ethTool, []string{"-g", intf})
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()

		if stderr.Len() > 0 {
			log.Print(stderr.String())
		}

		if err != nil {
			log.Print(err)
			continue
		}

		var max, cur bool
		var rxMax, txMax, rxCur, txCur string

		s := bufio.NewScanner(&stdout)
		for s.Scan() {
			line := s.Text()
			if line == "Pre-set maximums:" {
				max = true
				cur = false
			} else if line == "Current hardware settings:" {
				cur = true
				max = false
			} else if m := reRX.FindStringSubmatch(line); m != nil {
				if max {
					rxMax = m[1]
				} else if cur {
					rxCur = m[1]
				}
			} else if m := reTX.FindStringSubmatch(line); m != nil {
				if max {
					txMax = m[1]
				} else if cur {
					txCur = m[1]
				}
			}
		}
		if err := s.Err(); err != nil {
			log.Print(err)
			continue
		}

		var rxMaxInt, txMaxInt, rxCurInt, txCurInt int64

		if rxMax != "" {
			if rxMaxInt, err = strconv.ParseInt(rxMax, 10, 64); err != nil {
				log.Print(err)
				continue
			}
		}
		if txMax != "" {
			if txMaxInt, err = strconv.ParseInt(txMax, 10, 64); err != nil {
				log.Print(err)
				continue
			}
		}
		if rxCur != "" {
			if rxCurInt, err = strconv.ParseInt(rxCur, 10, 64); err != nil {
				log.Print(err)
				continue
			}
		}
		if txCur != "" {
			if txCurInt, err = strconv.ParseInt(txCur, 10, 64); err != nil {
				log.Print(err)
				continue
			}
		}

		args := []string{"-G", intf}
		initLen := len(args)

		if rxCurInt < rxMaxInt {
			args = append(args, "rx")
			args = append(args, rxMax)
		}

		if txCurInt < txMaxInt {
			args = append(args, "tx")
			args = append(args, txMax)
		}

		if len(args) <= initLen {
			// Nothing to do.
			continue
		}

		cmd = prepareCmd(ethTool, args)
		cmd.Stderr = &stderr
		err = cmd.Run()

		if stderr.Len() > 0 {
			log.Print(stderr.String())
		}

		if err != nil {
			log.Print(err)
			continue
		}
	}
}
