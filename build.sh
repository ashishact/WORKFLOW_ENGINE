go build -o bin/worker-server worker/main.go
go build -o bin/runtime-server start/main.go

GOOS=linux GOARCH=amd64 go build -o bin/worker-server-linux worker/main.go
GOOS=linux GOARCH=amd64 go build -o bin/runtime-server-linux start/main.go