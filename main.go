// Copyright © 2022 Zlatko Čalušić
//
// Use of this source code is governed by an MIT-style license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
)

const (
	sysClassNet = "/sys/class/net"
)

func main() {
	devices, err := ioutil.ReadDir(sysClassNet)
	if err != nil {
		log.Fatal(err)
	}

	for _, link := range devices {
		fullpath := path.Join(sysClassNet, link.Name())
		dev, err := os.Readlink(fullpath)
		if err != nil {
			continue
		}

		// Skip virtual devices.
		if strings.HasPrefix(dev, "../../devices/virtual/") {
			continue
		}

		fmt.Printf("%s\n", link.Name())
	}
}
