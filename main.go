package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	
	"github.com/rs/cors"
	"github.com/zenazn/goji/web"

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

	mux.Get(prefix + "/health", health())
	mux.Post(prefix + "/test", test())

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
		fmt.Println("Inserting record very slowly...")

		var id int64
		q := `INSERT INTO feedback(id, message, email) VALUES(DEFAULT, $1, 'test@friendy.me') returning id;`
		if err := dbInstance.QueryRowContext(context.Background(), q, in.Msg).Scan(&id); err != nil {
			log.Fatalln("Error inserting record: %s", err.Error())
			return
		}

		fmt.Println("Inserted new record. ID:", id)
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