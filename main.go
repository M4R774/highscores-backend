package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	_ "modernc.org/sqlite"
)

// Constants
var PORT int = 8080

// Objects
var database *sql.DB

func main() {
	fmt.Println("Starting application...")

	fmt.Println("Opening database...")
	open_database_connection()
	defer database.Close()

	http.HandleFunc("/highscores/", API_endpoint)

	fmt.Println("Starting http server on port", PORT)
	http.ListenAndServe("0.0.0.0:"+strconv.Itoa(PORT), nil)
}

func API_endpoint(writer http.ResponseWriter, request *http.Request) {
	request_url_path := fmt.Sprintf("%#v", request.URL.Path)
	game_name := request_url_path[13 : len(request_url_path)-1]
	game_name = sanitize_input(game_name)

	switch request.Method {
	case "GET":
		fmt.Fprint(writer, get_high_scores(game_name))
	case "POST":
		name, score := request.FormValue("name"), request.FormValue("score")
		score_int, err := strconv.Atoi(score)

		if err != nil {
			fmt.Println("Error during conversion")
			return
		}
		create_table_if_not_exists(game_name)
		fmt.Fprint(writer, add_high_score(name, score_int, game_name))
	default:
		fmt.Fprintf(writer, "Only GET and POST methods are supported.")
	}
}

func get_high_scores(game_name string) string {
	stmt, err := database.Prepare(fmt.Sprint("SELECT name, score FROM ", game_name, " ORDER BY score DESC"))
	if err != nil {
		panic(err)
	}
	defer stmt.Close()
	fmt.Println(game_name)
	rows, err := stmt.Query(game_name)
	if err != nil {
		return "No high scores for " + game_name
	}
	defer rows.Close()
	scores_string := ""
	for rows.Next() {
		var name string
		var score int
		err = rows.Scan(&name, &score)
		if err != nil {
			panic(err)
		}
		scores_string += fmt.Sprintf("%-20s %d\n", name, score)
	}
	return scores_string
}

func add_high_score(name string, score int, game_name string) string {
	if number_of_high_scores(game_name) >= 10 && score <= lowest_score(game_name) {
		fmt.Println(name, "tried to submit score", score, "in", game_name, "but it was not high enough")
		return "Your score is not high enough to reach top 10."
	} else {
		fmt.Println("Adding high score:", name, "with score:", score, "in", game_name)
		stmt, err := database.Prepare(fmt.Sprint("INSERT INTO ", game_name, " (name, score) VALUES (?, ?)"))
		if err != nil {
			panic(err)
		}
		defer stmt.Close()
		name = cut_string_to_length(name)
		_, err = stmt.Exec(name, score)
		if err != nil {
			panic(err)
		}
	}
	if number_of_high_scores(game_name) > 10 {
		delete_lowest_score(game_name)
	}
	return "Successfully added high score."
}

func lowest_score(game_name string) int {
	stmt, err := database.Prepare(fmt.Sprint("SELECT MIN(score) FROM ", game_name))
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

func number_of_high_scores(game_name string) int {
	stmt, err := database.Prepare(fmt.Sprint("SELECT COUNT(*) FROM ", game_name))
	if err != nil {
		panic(err)
	}
	defer stmt.Close()
	rows, err := stmt.Query(game_name)
	if err != nil {
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

func cut_string_to_length(string_to_cut string) string {
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

func delete_lowest_score(game_name string) {
	stmt, err := database.Prepare(fmt.Sprint("DELETE FROM ", game_name, " WHERE rowid = (SELECT rowid FROM ", game_name, " WHERE score = ? LIMIT 1)"))
	if err != nil {
		panic(err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(lowest_score(game_name))
	if err != nil {
		panic(err)
	}
}

func open_database_connection() *sql.DB {
	var err error
	if database == nil {
		database, err = sql.Open("sqlite", ":memory:")
		if err != nil {
			panic(err)
		}
	}
	var version string
	err = database.QueryRow("SELECT SQLITE_VERSION()").Scan(&version)
	if err != nil {
		panic(err)
	}
	fmt.Println("Database connection opened. SQLite version: ", version)
	return database
}

func sanitize_input(str string) string {
	var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)
	str = nonAlphanumericRegex.ReplaceAllString(str, "")
	return fmt.Sprintf("%q", str)
}

func create_table_if_not_exists(game_name string) {
	stmt, err := database.Prepare(fmt.Sprint("CREATE TABLE IF NOT EXISTS ", game_name, " (name TEXT, score INTEGER)"))
	if err != nil {
		panic(err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(game_name)
	if err != nil {
		panic(err)
	}
}
