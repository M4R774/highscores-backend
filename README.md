# highscores-backend

Generic back end service for saving and serving high scores for any game.

By default the service uses port `80` for dev environment. For production,
ports `80` and `443` are used.

## How to run

1. Create file with name `DEV_ENV` **or** create `config.json` file (see the `example.config.json`)
2. Run:

```bash
go run .
```

By default, the service will run with TLS. By creating a file called
`DEV_ENV` in the current directory, the service will not use TLS.

To run inside Docker create the config.json and run `./deploy.sh`.

## API description

GET `localhost/highscores/[game_name]` returns list of the highscores for the given game.

GET `localhost/highscores/[game_name]?json` returns list of the highscores for the given game
in json format.

GET `localhost/highscores/[game_name]?score=[score_to_check]` returns `true`/`false`
 if the given score is high enough for the given game. Useful for example when you want to
  ask player nickname only in the case that the score is high enough for the listing.

POST `localhost/highscores/[game_name]` body: `score=[player_score]&name=[player_nickname]`
