#!/bin/bash
git pull
docker build -t highscores-backend .
docker kill highscores-backend
docker rm highscores-backend
docker run -d -p 80:8080 --name highscores-backend --restart=always highscores-backend
