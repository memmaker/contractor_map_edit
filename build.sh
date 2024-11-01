#!/usr/bin/env bash
rm -rf builds
mkdir -p builds/mac
mkdir -p builds/win
#mkdir -p builds/linux
#mkdir -p builds/wasi

GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags '-s -w' -o ./builds/mac/mapedit .

GOOS=windows GOARCH=amd64 go build -trimpath -ldflags '-s -w' -o ./builds/win/mapedit.exe .
