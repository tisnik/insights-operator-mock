#!/bin/bash

go get github.com/fzipp/gocyclo/cmd/gocyclo
gocyclo -over 9 -avg .
