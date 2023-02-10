#!/bin/bash
git pull
CGO_ENABLED=0 GOARCH=arm go build \
    -ldflags='-w -s -extldflags "-static"' .
docker build -t highscores-backend .
docker kill highscores-backend
docker rm highscores-backend
docker run -d -p 8080:8080 --name highscores-backend --restart=always highscores-backend
