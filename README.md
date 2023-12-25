# asdf

[![Go](https://github.com/0dayfall/asdf/actions/workflows/go.yml/badge.svg)](https://github.com/0dayfall/asdf/actions/workflows/go.yml)
[![GoDoc](https://godoc.org/github.com/0dayfall/asdf?status.png)](https://godoc.org/github.com/0dayfall/asdf)
[![license](http://img.shields.io/badge/license-GNU3-red.svg?)](https://raw.githubusercontent.com/0dayfall/asdf/LICENSE)

## Description

This is a web finger server, see [RFC7033](https://datatracker.ietf.org/doc/html/rfc7033)

## Installation

```bash
git clone https://github.com/0dayfall/asdf.git
cd asdf
```

## Configuration

Use openssl to generate certificates
```
openssl genrsa -out server.key 2048
openssl req -new -x509 -sha256 -key server.key -out server.crt -days 365
```
Configure the environment variables in .env

## Running
```
docker-compose up --build
```
