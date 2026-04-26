#/bin/sh
# Install script for the host machine
# a simple go build and move to the local bin directory
go build -o ./dist/ ./cmd/lazy-click
cp ./dist/lazy-click $HOME/.local/bin/lazy-click
