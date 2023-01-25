# highscores-backend

Generic back end service for saving and serving high scores for any game.

## How to use

By default the service uses port 8080.

To run, just type:

```bash
go run .
```

To run inside Docker, see the deploy.sh script.

GET `localhost:8080/highscores/[game_name]` returns list of the highscores for the given game.

GET `localhost:8080/highscores/[game_name]?score=[score_to_check]` returns "true"/"false"
 if the given score is high enough for the given game. Useful for example when you want to
  ask player nickname only in the case that the score is high enough for the listing.

POST `localhost:8080/highscores/[game_name]` body: `score=[player_score]&name=[player_nickname]`
