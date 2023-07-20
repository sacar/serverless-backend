.PHONY: build deploy gomodgen

build:
	go mod download && GOOS=linux GOARCH=amd64 go build -o main

deploy:
	serverless deploy

remove:
	serverless remove

