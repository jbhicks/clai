# Makefile for the clai project

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

install:
	$(GOINSTALL) ./cmd/clai

dev: build
	DEBUG=true ./$(BINARY_NAME)

.PHONY: all build run test clean install dev
