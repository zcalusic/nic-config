// Copyright © 2022 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"io/ioutil"
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

func main() {
	devices, err := ioutil.ReadDir(sysClassNet)
	if err != nil {
		log.Fatal(err)
	}

	for _, link := range devices {
		intf := link.Name()
		fullpath := path.Join(sysClassNet, intf)
		dev, err := os.Readlink(fullpath)
		if err != nil {
			log.Print(err)
			continue
		}

		// Skip virtual devices.
		if strings.HasPrefix(dev, "../../devices/virtual/") {
			continue
		}

		log.Printf("Running %s -g %s\n", ethTool, intf)

		var stdout, stderr bytes.Buffer

		cmd := exec.Command(ethTool, "-g", intf)
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

		if rxMaxInt, err = strconv.ParseInt(rxMax, 10, 64); err != nil {
			log.Print(err)
			continue
		}
		if txMaxInt, err = strconv.ParseInt(txMax, 10, 64); err != nil {
			log.Print(err)
			continue
		}
		if rxCurInt, err = strconv.ParseInt(rxCur, 10, 64); err != nil {
			log.Print(err)
			continue
		}
		if txCurInt, err = strconv.ParseInt(txCur, 10, 64); err != nil {
			log.Print(err)
			continue
		}

		log.Printf("%s: rxmax %d txmax %d rxcur %d txcur %d", intf, rxMaxInt, txMaxInt, rxCurInt, txCurInt)
	}
}
