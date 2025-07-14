.PHONY: dev
# Live reload development
dev:
	wgo -verbose go run ./cmd/clai/main.go
# Makefile for the clai project
#
# Additional targets:
# minimal_testing_air: Runs minimal_testing.go with air for live prototyping

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GORUN=$(GOCMD) run
GOINSTALL=$(GOCMD) install
BINARY_NAME=clai
BINARY_UNIX=$(BINARY_NAME)

all: build

build:
	$(GOBUILD) -o $(BINARY_NAME) ./cmd/clai

run:
	$(GORUN) ./cmd/clai

test:
	$(GOTEST) ./...

clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)


prototype:
	$(GORUN) ./minimal_testing.go
