package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
	_ "modernc.org/sqlite"
)

type Database struct {
	mutex sync.Mutex
	db    *sql.DB
}

func main() {
	init_logging()
	log.Println("Starting application...")

	database := open_database_connection()
	defer database.db.Close()

	if !file_exists("DEV_ENV") {
		mux := http.NewServeMux()
		mux.HandleFunc("/highscores/", database.API_endpoint)

		domain := read_domain_from_config_file()

		log.Println("TLS domain:", domain+", www."+domain)
		certManager := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(domain, "www."+domain),
			//Cache:      autocert.DirCache("certs"),
		}

		tlsConfig := certManager.TLSConfig()
		server := http.Server{
			Addr:      ":443",
			Handler:   mux,
			TLSConfig: tlsConfig,
		}

		go http.ListenAndServe(":80", certManager.HTTPHandler(nil))
		log.Println("Server listening on", server.Addr)
		if err := server.ListenAndServeTLS("", ""); err != nil {
			fmt.Println(err)
		}
	} else {
		log.Println("Starting local testing server on port 80.")
		http.HandleFunc("/highscores/", database.API_endpoint)
		http.ListenAndServe(":80", nil)
	}
}

func file_exists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func read_domain_from_config_file() string {
	var payload map[string]interface{}
	if file_exists("config.json") {
		content, err := ioutil.ReadFile("./config.json")
		if err != nil {
			log.Fatal("Error when opening file: ", err)
		}
		err = json.Unmarshal(content, &payload)
		if err != nil {
			log.Fatal("Error during Unmarshal(): ", err)
		}
	} else {
		panic("Config file not found.")
	}
	return payload["domain"].(string)
}

func init_logging() {
	log.SetFlags(0)
	log.SetOutput(new(logWriter))
}

func (database *Database) API_endpoint(writer http.ResponseWriter, request *http.Request) {
	log.Println("Got request to /highscores")
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	database.mutex.Lock()
	defer database.mutex.Unlock()
	request_url_path := fmt.Sprintf("%#v", request.URL.Path)
	game_name := request_url_path[13 : len(request_url_path)-1]
	game_name = sanitize_input(game_name)
	switch request.Method {
	case "GET":
		query_parameters := request.URL.Query()
		if query_parameters.Has("score") {
			database.check_if_score_is_high_enough(query_parameters, writer, game_name)
		} else {
			fmt.Fprint(writer, database.get_high_scores(query_parameters, game_name))
		}
	case "POST":
		name, score := request.FormValue("name"), request.FormValue("score")
		score_int, err := strconv.Atoi(score)

		if err != nil {
			log.Println("Error during conversion.")
			fmt.Fprintf(writer, "Invalid score value. Must be an integer.")
			return
		}
		database.create_table_if_not_exists(game_name)
		fmt.Fprint(writer, database.add_high_score(name, score_int, game_name))
	default:
		fmt.Fprintf(writer, "Only GET and POST methods are supported.")
	}
}

func (database *Database) check_if_score_is_high_enough(query_parameters url.Values, writer http.ResponseWriter, game_name string) {
	score_parameter := query_parameters.Get("score")
	score, err := strconv.Atoi(score_parameter)
	if err != nil {
		log.Println("Error during conversion.")
		fmt.Fprintf(writer, "Invalid score value. Must be an integer.")
	} else {
		database.create_table_if_not_exists(game_name)
		if database.number_of_high_scores(game_name) < 10 || score > database.lowest_score(game_name) {
			fmt.Fprintf(writer, "true")
		} else {
			fmt.Fprintf(writer, "false")
		}
	}
}

func (database *Database) get_high_scores(query_parameters url.Values, game_name string) string {
	stmt, err := database.db.Prepare(fmt.Sprint("SELECT name, score FROM ", game_name, " ORDER BY score DESC"))
	if err != nil {
		panic(err)
	}
	defer stmt.Close()
	log.Println("Someone checked high scores for", game_name)
	rows, err := stmt.Query(game_name)
	if err != nil {
		log.Println(err)
		return "No high scores for " + game_name
	}
	defer rows.Close()
	scores_string := ""
	if query_parameters.Has("json") {
		position := 0
		var list_of_players []Player
		for rows.Next() {
			position++
			var name string
			var score int
			err = rows.Scan(&name, &score)
			if err != nil {
				panic(err)
			}
			new_player := Player{Name: name, Score: score, Position: position}
			list_of_players = append(list_of_players, new_player)
		}
		scores_bytes, _ := json.Marshal(list_of_players)
		scores_string = string(scores_bytes)
		log.Println(scores_string)
	} else {
		for rows.Next() {
			var name string
			var score int
			err = rows.Scan(&name, &score)
			if err != nil {
				panic(err)
			}
			scores_string += fmt.Sprintf("%-20s %d\n", name, score)
		}
	}
	return scores_string
}

type Player struct {
	Name     string `json:"name"`
	Score    int    `json:"score"`
	Position int    `json:"position"`
}

func (database *Database) add_high_score(name string, score int, game_name string) string {
	if database.number_of_high_scores(game_name) >= 10 && score <= database.lowest_score(game_name) {
		log.Println(name, "tried to submit score", score, "in", game_name, "but it was not high enough")
		return "Your score is not high enough to reach top 10."
	} else {
		log.Println("Adding high score:", name, "with score:", score, "in", game_name)
		stmt, err := database.db.Prepare(fmt.Sprint("INSERT INTO ", game_name, " (name, score) VALUES (?, ?)"))
		if err != nil {
			panic(err)
		}
		defer stmt.Close()
		name = database.cut_string_to_length(name)
		_, err = stmt.Exec(name, score)
		if err != nil {
			panic(err)
		}
	}
	if database.number_of_high_scores(game_name) > 10 {
		database.delete_lowest_score(game_name)
	}
	return "Successfully added high score."
}

func (database *Database) lowest_score(game_name string) int {
	if database.number_of_high_scores(game_name) == 0 {
		return math.MinInt // No score -> lowest possible score
	}
	stmt, err := database.db.Prepare(fmt.Sprint("SELECT MIN(score) FROM ", game_name))
	if err != nil {
		panic(err)
	}
	defer stmt.Close()
	rows, err := stmt.Query(game_name)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	var lowest_score int
	for rows.Next() {
		err = rows.Scan(&lowest_score)
		if err != nil {
			panic(err)
		}
	}
	return lowest_score
}

func (database *Database) number_of_high_scores(game_name string) int {
	stmt, err := database.db.Prepare(fmt.Sprint("SELECT COUNT(*) FROM ", game_name))
	if err != nil {
		panic(err)
	}
	defer stmt.Close()
	rows, err := stmt.Query()
	if err != nil {
		log.Println(err)
		panic(err)
	}
	defer rows.Close()
	var number_of_high_scores int
	for rows.Next() {
		err = rows.Scan(&number_of_high_scores)
		if err != nil {
			panic(err)
		}
	}
	return number_of_high_scores
}

func (database *Database) cut_string_to_length(string_to_cut string) string {
	truncated := ""
	count := 0
	for _, char := range string_to_cut {
		truncated += string(char)
		count++
		if count >= 20 {
			break
		}
	}
	return truncated
}

func (database *Database) delete_lowest_score(game_name string) {
	log.Println("Deleting highscore")
	stmt, err := database.db.Prepare(fmt.Sprint("DELETE FROM ", game_name, " WHERE rowid = (SELECT rowid FROM ", game_name, " WHERE score = ? LIMIT 1)"))
	if err != nil {
		panic(err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(database.lowest_score(game_name))
	if err != nil {
		panic(err)
	}
}

func open_database_connection() Database {

	sqlite_database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}
	database := Database{db: sqlite_database}

	var version string
	err = database.db.QueryRow("SELECT SQLITE_VERSION()").Scan(&version)
	if err != nil {
		panic(err)
	}
	log.Println("Database connection opened. SQLite version:", version)
	return database
}

func sanitize_input(str string) string {
	var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	str = nonAlphanumericRegex.ReplaceAllString(str, "")
	return fmt.Sprintf("'" + str + "'")
}

func (database *Database) create_table_if_not_exists(game_name string) {
	database.db.Exec(fmt.Sprint("CREATE TABLE IF NOT EXISTS ", game_name, " (name TEXT, score INTEGER)"))
}

type logWriter struct {
}

func (writer logWriter) Write(bytes []byte) (int, error) {
	return fmt.Print(time.Now().Format("02.01.2006 15:04:05") + " " + string(bytes))
}
