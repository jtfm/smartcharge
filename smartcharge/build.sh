rm -rf bin
mkdir -p bin
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin/bootstrap -v main.go
zip -j bin/app.zip bin/bootstrap
rm -rf bin/bootstrap
