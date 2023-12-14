go mod tidy
SET GOARCH=amd64
SET GOOS=windows
go build -v -o pingmon-%GOOS%-%GOARCH%.exe

SET GOARCH=amd64
SET GOOS=linux
go build -v -o pingmon-%GOOS%-%GOARCH%

SET GOARCH=arm64
SET GOOS=linux
go build -v -o pingmon-%GOOS%-%GOARCH%

SET GOARCH=riscv64
SET GOOS=linux
go build -v -o pingmon-%GOOS%-%GOARCH%
