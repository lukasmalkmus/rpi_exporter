# lukasmalkmus/rpi_exporter

> A Raspberry Pi CPU temperature exporter. - by **[Lukas Malkmus]**

[![Travis Status][travis_badge]][travis]
[![Coverage Status][coverage_badge]][coverage]
[![Go Report][report_badge]][report]
[![Latest Release][release_badge]][release]
[![License][license_badge]][license]
[![Docker][docker_badge]][docker]

---

## Table of Contents

1. [Introduction](#introduction)
2. [Usage](#usage)
3. [Contributing](#contributing)
4. [License](#license)

### Introduction

The *rpi_exporter* is a simple server that scrapes the Raspberry Pi's CPU
temperature and exports it via HTTP for Prometheus consumption.

### Usage

#### Installation

The easiest way to run the *rpi_exporter* is by grabbing the latest binary from
the [release page][release].

##### Building from source

This project uses [dep] for vendoring.

```bash
mkdir -p $GOPATH/src/github.com/lukasmalkmus/rpi_exporter
git clone https://github.com/lukasmalkmus/rpi_exporter.git $GOPATH/src/github.com/lukasmalkmus/rpi_exporter
cd $GOPATH/src/github.com/lukasmalkmus/rpi_exporter
go build
# or promu build
```

#### Using the application

```bash
./rpi_exporter [flags]
```

Help on flags:

```bash
./rpi_exporter --help
```

#### Docker images

Thanks to [Carlos Eduardo] docker images are now available for this exporter!
Checkout [DockerHub]!

### Contributing

Feel free to submit PRs or to fill Issues. Every kind of help is appreciated.

### License

© Lukas Malkmus, 2018

Distributed under Apache License (`Apache License, Version 2.0`).

See [LICENSE](LICENSE) for more information.

<!-- Links -->
[dep]: https://github.com/golang/dep
[Lukas Malkmus]: https://github.com/lukasmalkmus
[Carlos Eduardo]: https://github.com/carlosedp
[DockerHub]: https://hub.docker.com/r/carlosedp/arm_exporter

<!-- Badges -->
[travis]: https://travis-ci.org/lukasmalkmus/rpi_exporter
[travis_badge]: https://travis-ci.org/lukasmalkmus/rpi_exporter.svg
[coverage]: https://coveralls.io/github/lukasmalkmus/rpi_exporter?branch=master
[coverage_badge]: https://coveralls.io/repos/github/lukasmalkmus/rpi_exporter/badge.svg?branch=master
[report]: https://goreportcard.com/report/github.com/lukasmalkmus/rpi_exporter
[report_badge]: https://goreportcard.com/badge/github.com/lukasmalkmus/rpi_exporter
[release]: https://github.com/lukasmalkmus/rpi_exporter/releases
[release_badge]: https://img.shields.io/github/release/lukasmalkmus/rpi_exporter.svg
[license]: https://opensource.org/licenses/Apache-2.0
[license_badge]: https://img.shields.io/badge/license-Apache-blue.svg
[docker]: https://hub.docker.com/r/carlosedp/arm_exporter
[docker_badge]: https://img.shields.io/docker/pulls/carlosedp/arm_exporter.svg