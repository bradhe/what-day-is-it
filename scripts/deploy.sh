#!/bin/bash

HOST=$1

GOOS=linux GOARCH=amd64 go build .
scp what-day-is-it what-day-is-it:what-day-is-it
ssh $1 'sudo mv what-day-is-it /usr/local/bin/what-day-is-it && sudo service what-day-is-it restart'
