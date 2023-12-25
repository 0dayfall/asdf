# asdf

[![Go](https://github.com/0dayfall/asdf/actions/workflows/go.yml/badge.svg)](https://github.com/0dayfall/asdf/actions/workflows/go.yml)
[![GoDoc](https://godoc.org/github.com/0dayfall/asdf/main?status.png)](https://godoc.org/github.com/0dayfall/asdf/main)
[![Coverage Status](https://coveralls.io/repos/0dayfall/asdf/main/badge.png?branch=master)](https://coveralls.io/r/0dayfall/asdf/main?branch=main)
[![license](http://img.shields.io/badge/license-GNU3-red.svg?)](https://raw.githubusercontent.com/0dayfall/asdf/main/LICENSE)

## Description

This is a web finger server, see [RFC7033](https://datatracker.ietf.org/doc/html/rfc7033)

## Installation

```bash
git clone https://github.com/0dayfall/asdf.git
cd asdf

## Configuration

Use openssl to generate certificates
```
openssl genrsa -out server.key 2048
openssl req -new -x509 -sha256 -key server.key -out server.crt -days 365
```

## Running
docker-compose up --build
```

Configure the environment variables in .env
