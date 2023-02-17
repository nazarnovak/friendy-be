package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/rs/cors"
	"github.com/zenazn/goji/web"
)

type Incoming struct {
	Msg string `json:"msg"`
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
	}
}
  
func main() {
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

	mux.Use(getCorsHandler())

	mux.Post("/api/test", test())

	//mux.Get("/api/test", Test())

	return mux
}

func getCorsHandler() func(http.Handler) http.Handler {
	allowedOrigins := []string{}
// TODO: Add mode dev + mode prod here to separate sites
	allowedOrigins = append(allowedOrigins, "http://localhost:3000")
	// External IP
	// allowedOrigins = append(allowedOrigins, "http://84.219.232.19:3000")

	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedHeaders:   []string{"Accept", "Authorization", "Cache-Control", "Content-Type", "Origin", "User-Agent", "Viewport", "X-Requested-With"},
		MaxAge:           1728000,
		AllowCredentials: true,
		AllowedMethods:   []string{"GET", "POST"},
	})

	return c.Handler
}