package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
	_ "modernc.org/sqlite"
)

const (
	httpPort = "127.0.0.1:8080"
)

var (
	flgProduction          = true
	flgRedirectHTTPToHTTPS = true
)

type Database struct {
	mutex sync.Mutex
	db    *sql.DB
}

func main() {
	log.SetFlags(0)
	log.SetOutput(new(logWriter))
	log.Println("Opening database...")
	database := open_database_connection()
	defer database.db.Close()

	parseFlags()
	var m *autocert.Manager

	var httpsSrv *http.Server
	if flgProduction {
		hostPolicy := func(ctx context.Context, host string) error {
			// Note: change to your real host
			allowedHost := "martta.tk"
			if host == allowedHost {
				return nil
			}
			return fmt.Errorf("acme/autocert: only %s host is allowed", allowedHost)
		}

		dataDir := "."
		m = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: hostPolicy,
			Cache:      autocert.DirCache(dataDir),
		}

		httpsSrv = makeHTTPServer()
		httpsSrv.Addr = ":443"
		httpsSrv.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}

		go func() {
			fmt.Printf("Starting HTTPS server on %s\n", httpsSrv.Addr)
			err := httpsSrv.ListenAndServeTLS("", "")
			if err != nil {
				log.Fatalf("httpsSrv.ListendAndServeTLS() failed with %s", err)
			}
		}()
	}

	var httpSrv *http.Server
	if flgRedirectHTTPToHTTPS {
		httpSrv = makeHTTPToHTTPSRedirectServer()
	} else {
		httpSrv = makeHTTPServer()
	}
	// allow autocert handle Let's Encrypt callbacks over http
	if m != nil {
		httpSrv.Handler = m.HTTPHandler(httpSrv.Handler)
	}
	httpSrv.Addr = httpPort
	fmt.Printf("Starting HTTP server on %s\n", httpPort)
	err := httpSrv.ListenAndServe()
	if err != nil {
		log.Fatalf("httpSrv.ListenAndServe() failed with %s", err)
	}
}

func (database *Database) API_endpoint(writer http.ResponseWriter, request *http.Request) {
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
			fmt.Fprint(writer, database.get_high_scores(game_name))
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

func (database *Database) get_high_scores(game_name string) string {
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

func makeServerFromMux(mux *http.ServeMux) *http.Server {
	// set timeouts so that a slow or malicious client doesn't
	// hold resources forever
	return &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
		Handler:      mux,
	}
}

func makeHTTPServer() *http.Server {
	mux := &http.ServeMux{}
	mux.HandleFunc("/higscores/", API_endpoint)
	return makeServerFromMux(mux)
}

func makeHTTPToHTTPSRedirectServer() *http.Server {
	handleRedirect := func(w http.ResponseWriter, r *http.Request) {
		newURI := "https://" + r.Host + r.URL.String()
		http.Redirect(w, r, newURI, http.StatusFound)
	}
	mux := &http.ServeMux{}
	mux.HandleFunc("/", handleRedirect)
	return makeServerFromMux(mux)
}

func parseFlags() {
	flag.BoolVar(&flgProduction, "production", false, "if true, we start HTTPS server")
	flag.BoolVar(&flgRedirectHTTPToHTTPS, "redirect-to-https", false, "if true, we redirect HTTP to HTTPS")
	flag.Parse()
}
