#!/bin/sh
# Native Windows build (Git Bash): embeds the icon (GLFW already brings a manifest) and uses the GUI
# subsystem so no console window opens next to the app.
# Needs Go and a MinGW gcc (e.g. winget BrechtSanders.WinLibs.POSIX.UCRT) on
# PATH, plus: go install github.com/tc-hib/go-winres@latest
set -e
cd "$(dirname "$0")/.."
"$(go env GOPATH)/bin/go-winres" simply --arch amd64 --icon winres/icon.png \
	--manifest none --product-name "Nelko P21 Print" --file-description "Nelko P21 Print" \
	--out cmd/nelko-print/rsrc
CGO_ENABLED=1 go build -ldflags "-H=windowsgui" -o nelko-print.exe ./cmd/nelko-print
