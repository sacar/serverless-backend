.PHONY: build build-thumbnail deploy gomodgen

build:
	go mod download && cd src && GOOS=linux GOARCH=amd64 go build -o ../bin/main

build-thumbnail:
	go mod download && cd thumbnail-generation && GOOS=linux GOARCH=amd64 go build -o ../bin/putItemTriggeredFunction

deploy:
	serverless deploy

remove:
	serverless remove

