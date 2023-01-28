#!/bin/bash
git pull
docker build -t highscores-backend .
docker kill highscores-backend
docker rm highscores-backend
docker run -d -p 443:443 --name highscores-backend --restart=always highscores-backend
