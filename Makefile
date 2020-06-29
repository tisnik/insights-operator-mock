.PHONY: help clean build fmt lint vet run test style cyclo

SOURCES:=$(shell find . -name '*.go')

default: build

clean: ## Run go clean
	@go clean

build: ## Run go build
	@go build
