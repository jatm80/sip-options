#!/bin/bash

rm build/sip_options
env GOOS=linux GOARCH=amd64 go build -o build/sip_options . 