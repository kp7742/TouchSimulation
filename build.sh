#!/bin/bash
go clean
# GOOS=linux CGO_ENABLED=0 GOARCH=arm GOARM=7 go build -trimpath -gcflags "-trimpath $PWD" -ldflags "-s -w" -o "bin/TouchTest"
GOOS=linux CGO_ENABLED=0 GOARCH=arm64 go build -trimpath -gcflags "-trimpath $PWD" -ldflags "-s -w" -o "bin/TouchTest64"