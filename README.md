# asdf [![Go](https://github.com/0dayfall/asdf/actions/workflows/go.yml/badge.svg)](https://github.com/0dayfall/asdf/actions/workflows/go.yml)

## Description

This is a web finger server, see [RFC7033](https://datatracker.ietf.org/doc/html/rfc7033)

## Installation

```bash
git clone https://github.com/0dayfall/asdf.git
cd asdf

## Configuration

Use to generate a string that can be used as input to AES256 
```
openssl rand -base64 32
```

set the SESSION_KEY environment variable

## Running
docker-compose up --build
```

Configure the environment variables in .env
