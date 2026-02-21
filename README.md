# PbT (People by Temperament) in Go

This repository contains a Go reimplementation of the PbT prototype originally provided under `python/`.

## What is implemented

- registration, login, logout
- profile storage/editing (town/country/mottos/likes/dislikes/GPS/enneagram)
- TTT questionnaire and MBTI-style evaluation with tie behavior equivalent to the Python version
- member search with filters (contact state, country, motto text, TTT type, max distance)
- RCD flow (send, accept, reject)
- status pages for own and foreign profiles
- member plot image (`/members/plot.png`)

## Debian setup

```bash
sudo apt update
sudo apt install -y golang-go
```

(Go 1.22+ recommended.)

## Build

From repository root:

```bash
go build ./...
```

## Run

```bash
go run .
```

Then open: <http://127.0.0.1:8080/>

## Test

```bash
go test ./...
```
