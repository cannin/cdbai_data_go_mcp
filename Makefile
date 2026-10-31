APP_NAME := cdbai_data_go_mcp
CMD := ./cmd/cdbai_data_go_mcp
DIST := dist

.PHONY: all test release current clean

all: test release current

test:
	go test ./...
	go vet ./...

release:
	rm -rf $(DIST)
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o $(DIST)/$(APP_NAME)-linux-amd64 $(CMD)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -o $(DIST)/$(APP_NAME)-darwin-arm64 $(CMD)

current:
	mkdir -p $(DIST)
	go build -trimpath -o $(DIST)/$(APP_NAME) $(CMD)

clean:
	rm -rf $(DIST)
