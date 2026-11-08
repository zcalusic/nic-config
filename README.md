# NIC config

[![Build Status](https://github.com/zcalusic/nic-config/actions/workflows/go.yml/badge.svg)](https://github.com/zcalusic/nic-config/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/zcalusic/nic-config.svg)](https://pkg.go.dev/github.com/zcalusic/nic-config)
[![License](https://img.shields.io/badge/license-MIT-a31f34.svg?maxAge=2592000)](https://github.com/zcalusic/nic-config/blob/master/LICENSE)
[![Powered by](https://img.shields.io/badge/powered_by-Go-5272b4.svg?maxAge=2592000)](https://go.dev/)
[![Platform](https://img.shields.io/badge/platform-Linux-009bde.svg?maxAge=2592000)](https://www.linuxfoundation.org/)

NIC config is a very simple utility that will automatically tune all physical network interfaces in a Linux machine for best performance. Currently it knows how to increase rx/tx ring parameters to their maximum value, which decreases the number of dropped packets when the network load is high. It talks to the kernel's ethtool interface directly, so the ```ethtool``` utility is not needed: first it checks what are the current hardware settings, then compares them to pre-set maximums, and finally applies optimized settings, if needed.

Virtual interfaces are skipped, as are physical interfaces whose driver has no ring parameters to tune.

## Installation

```
go install github.com/zcalusic/nic-config@latest
```

Sample systemd service file is also provided, it expects the binary at ```/usr/local/sbin/nic-config```. Changing NIC ring parameters shortly interrupts the network connection (depending on the NIC driver), so it's best to run the utility before the systemd network target, very early in the boot sequence.

```
sudo install -m 755 "$(go env GOPATH)/bin/nic-config" /usr/local/sbin/
sudo install -m 644 etc/nic-config.service /etc/systemd/system/
sudo systemctl enable nic-config
```

Interfaces that appear after the utility has run, like hotplugged NICs, are not tuned until it runs again.

## Usage

```
nic-config [-h] [-version] [-n]
```

Without arguments, it tunes all interfaces right away, which requires root or the ```CAP_NET_ADMIN``` capability. Use ```-n``` to see what would change, without changing it.

If tuning an interface fails, the error is logged, the remaining interfaces are still tuned, and the exit status is 1. The exit status is 2 for invalid arguments.

## Sample output

For a typical NIC with default settings like this:

```
Ring parameters for enp7s0:
Pre-set maximums:
RX:		4096
RX Mini:	n/a
RX Jumbo:	n/a
TX:		4096
Current hardware settings:
RX:		256
RX Mini:	n/a
RX Jumbo:	n/a
TX:		256
```

The utility would apply the optimal settings, and log the change:

```
enp7s0: rx 256 -> 4096 | tx 256 -> 4096
```

Nothing is logged for interfaces that are already at their maximum values.

## License

```
The MIT License (MIT)

Copyright © 2022 Zlatko Čalušić

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```
