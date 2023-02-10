# highscores-backend

Generic back end service for saving and serving high scores for any game.

By default the service uses port `8080`. For TLS (HTTPS), ports `8080` and
`8443` are used.

## How to run

1. Optional step, skip if TLS is not needed: create empty file `TLS_ENABLED`
 and `config.json` file (see the `example.config.json`)
2. Run:

```bash
go run .
```

By default, the service will run without TLS. By creating a file called
`TLS_ENABLED` in the current directory, the service will use TLS.

To run inside Docker create the config.json and run `./deploy.sh`.

## API description

GET `localhost:8080/highscores/[game_name]` returns list of the highscores for the given game.

GET `localhost:8080/highscores/[game_name]?json` returns list of the highscores for the given game
in json format.

GET `localhost:8080/highscores/[game_name]?score=[score_to_check]` returns `true`/`false`
 if the given score is high enough for the given game. Useful for example when you want to
  ask player nickname only in the case that the score is high enough for the listing.

POST `localhost:8080/highscores/[game_name]` body: `score=[player_score]&name=[player_nickname]`
