build:
	GOOS=linux GOARCH=amd64 go build -o handler main.go
	zip handler.zip handler

clean:
	rm -f handler handler.zip debug-*.yaml