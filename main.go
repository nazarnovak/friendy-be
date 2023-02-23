package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/rs/cors"
	"github.com/zenazn/goji/web"

	"github.com/gorilla/websocket"

	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
)

var dbInstance *sql.DB

type Incoming struct {
	Msg string `json:"msg"`
}

// TODO:
// 1. Change GoogleCloudPlatform postgres driver to something else, cus it's Digital Ocean now. Duh
//

func main() {
	if err := InitDB(); err != nil {
		log.Fatalln(fmt.Sprintf("Error initializing DB: %s", err.Error()))
	}

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router(),
	}

	fmt.Println("Running on port :8080")

	// Start the server and log any errors it returns
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(fmt.Sprintf("error running server: %s", err.Error()))
	}
}

func router() *web.Mux {
	mux := web.New()

	// TODO: Append this later. For now, DO hosts BE under /api, so I'll skip it
	//prefix := "/api"
	prefix := ""

	mux.Use(getCorsHandler())

	mux.Get(prefix+"/health", health())
	mux.Post(prefix+"/test", test())
	mux.Get(prefix+"/ws", ws())

	//mux.Get("/api/test", Test())

	return mux
}

func getCorsHandler() func(http.Handler) http.Handler {
	allowedOrigins := []string{}
	// TODO: Add mode dev + mode prod here to separate sites
	allowedOrigins = append(allowedOrigins, "http://localhost:3000")

	// Home IP
	allowedOrigins = append(allowedOrigins, "http://78.82.194.129")

	// External IP
	allowedOrigins = append(allowedOrigins, "https://friendy-fe-kkrep.ondigitalocean.app",
		"http://friendy.me", "https://friendy.me")

	// Allow all for now
	//allowedOrigins = append(allowedOrigins, "*")

	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedHeaders:   []string{"Accept", "Authorization", "Cache-Control", "Content-Type", "Origin", "User-Agent", "Viewport", "X-Requested-With"},
		MaxAge:           1728000,
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST"},
	})

	return c.Handler
}

func health() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("I'm alive")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Bloop"))
		return
	}
}

func test() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)

		var in Incoming
		err := decoder.Decode(&in)
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Println(in.Msg)
		var todaysRecords int
		q := `SELECT COUNT(*) FROM feedback WHERE created::DATE = $1 AND email = 'test@friendy.me';`
		err = dbInstance.QueryRowContext(context.Background(), q, time.Now().UTC().Format("2006-01-02")).Scan(&todaysRecords)
		if err != nil && err != sql.ErrNoRows {
			log.Fatalln("Error selecting records: %s", err.Error())
			return
		}

		// We already have our daily 2 records, stopppp
		if todaysRecords >= 2 {
			fmt.Println("Already have 2 records, chief. Skipping!")
			return
		}

		fmt.Println("Inserting record very slowly...")

		var id int64
		q = `INSERT INTO feedback(id, message, email,created) VALUES(DEFAULT, $1, 'test@friendy.me', $2) returning id;`
		if err := dbInstance.QueryRowContext(context.Background(), q, in.Msg, time.Now().UTC()).Scan(&id); err != nil {
			log.Fatalln("Error inserting record: %s", err.Error())
			return
		}

		fmt.Println("Inserted new record. ID:", id)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func ws() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Hit ws")
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatalln("Error upgrading connection to websockets: %s", err.Error())
			return
		}

		// helpful log statement to show connections
		log.Println("Client Connected")

		reader(c)
	}
}

func reader(conn *websocket.Conn) {
	for {
		// read in a message
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		// print out that message for clarity
		fmt.Println(string(p))

		var todaysRecords int
		q := `SELECT COUNT(*) FROM feedback WHERE created::DATE = $1 AND email='websocket.test@friendy.me';`
		err = dbInstance.QueryRowContext(context.Background(), q, time.Now().UTC().Format("2006-01-02")).Scan(&todaysRecords)
		if err != nil && err != sql.ErrNoRows {
			log.Fatalln("Error selecting records: %s", err.Error())
			return
		}

		// We already have our daily 2 records, stopppp
		if todaysRecords >= 2 {
			fmt.Println("Already have 2 websocket records, chief. Skipping!")
			return
		}

		fmt.Println("Inserting record very slowly...")

		var id int64
		q = `INSERT INTO feedback(id, message, email,created) VALUES(DEFAULT, $1, 'websocket.test@friendy.me', $2) returning id;`
		if err := dbInstance.QueryRowContext(context.Background(), q, string(p), time.Now().UTC()).Scan(&id); err != nil {
			log.Fatalln("Error inserting websocket record:", err.Error())
			return
		}

		fmt.Println("Inserted new websocket record. ID:", id)

		if err := conn.WriteMessage(messageType, p); err != nil {
			log.Println(err)
			return
		}

	}
}

func InitDB() error {
	// connectionString := "postgres://nazarnovak@localhost/friendy?sslmode=disable"
	connectionString := "postgresql://friendy:AVNS_LQ-aLzuy6Rrnrh-w9Cr@app-9b093242-2b97-42ae-b8fc-f684131dfcd5-do-user-13379982-0.b.db.ondigitalocean.com:25060/friendy?sslmode=require"
	driver := "postgres"

	// if !isDev {
	// 	connectionString = cnfDB.ConnectionProd
	// 	driver = "cloudsqlpostgres"
	// }

	db, err := sql.Open(driver, connectionString)
	if err != nil {
		return err
	}
	//defer db.Close()

	if err := db.Ping(); err != nil {
		return err
	}

	dbInstance = db

	// Supress the "ephemeral certificate for instance hobeechat:europe-west6:myinstance3 will expire soon, refreshing now."
	//logging.LogVerboseToNowhere()

	return nil
}
