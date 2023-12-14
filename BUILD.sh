#! /bin/bash
go mod tidy
GOARCH=amd64 GOOS=windows go build -v -o pingmon-$GOOS-$GOARCH.exe
GOARCH=amd64 GOOS=linux go build -v -o pingmon-$GOOS-$GOARCH
GOARCH=arm64 GOOS=linux go build -v -o pingmon-$GOOS-$GOARCH
GOARCH=riscv64 GOOS=linux go build -v -o pingmon-$GOOS-$GOARCH
