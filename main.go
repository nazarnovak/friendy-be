package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rs/cors"
	"github.com/zenazn/goji/web"

	"github.com/gorilla/websocket"

	stripe "github.com/stripe/stripe-go/v74"
	paymentintent "github.com/stripe/stripe-go/v74/paymentintent"

	"github.com/dukex/mixpanel"

	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
)

var dbInstance *sql.DB

var mixpanelClient mixpanel.Mixpanel

var trackCodeToMessage = map[int]string{
	1: "1.Landing",
	2: "2.Sign up",
	3: "3.Payment",
	4: "4.Payment attempt",
	5: "5.Welcome",
	6: "6.Profile",
	7: "7.Your values",
	8: "8.Friend values",
	9: "9.Searching",
	10: "10.Chat",
}

type TrackRequest struct {
	Event int `json:"event"`
}

type Incoming struct {
	Msg string `json:"msg"`
}

func main() {
	stripeApiKey := os.Getenv("STRIPE_SECRET_KEY")
	if stripeApiKey == "" {
		log.Fatalln(fmt.Sprintf("Stripe secret key not set"))
	}

	mixpanelProjectToken := os.Getenv("MIXPANEL_PROJECT_TOKEN")
	if mixpanelProjectToken == "" {
		log.Fatalln(fmt.Sprintf("Mixpanel project token not set"))
	}

	mixpanelClient = mixpanel.New(mixpanelProjectToken, "")

	if err := InitDB(); err != nil {
		log.Fatalln(fmt.Sprintf("Error initializing DB: %s", err.Error()))
	}

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router(stripeApiKey),
	}

	fmt.Println("Running on port :8080")

	// Start the server and log any errors it returns
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(fmt.Sprintf("error running server: %s", err.Error()))
	}
}

func router(stripeApiKey string) *web.Mux {
	mux := web.New()

	// TODO: Append this later. For now, DO hosts BE under /api, so I'll skip it
	//prefix := "/api"
	prefix := ""

	mux.Use(getCorsHandler())

	mux.Get(prefix+"/health", health())
	mux.Get(prefix+"/stripe", stripeSecret(stripeApiKey))
	mux.Post(prefix+"/track", track())
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
	}
}

type StripeResponse struct {
	ClientSecret string `json:"clientSecret"`
}

type HTTPError struct {
	ErrorMessage string `json:"errorMessage"`
}

func track() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		decoder := json.NewDecoder(r.Body)

		var tr TrackRequest
		err := decoder.Decode(&tr)
		if err != nil {
			fmt.Println(err)
			return
		}

		eventMsg, ok := trackCodeToMessage[tr.Event]
		if !ok {
			fmt.Println("Failed to find event with number:", tr.Event)
			return
		}

		// TODO: id will be cookie or something identifiable here
		// Can also include properties as last parameter
		// &mixpanel.Event{
		// 	Properties: map[string]interface{}{
		// 		"from": "email@email.com",
		// 	},
		// }
		err = mixpanelClient.Track("id", eventMsg, &mixpanel.Event{
			Properties: map[string]interface{}{},
		})
		if err != nil {
			fmt.Println("Failed to send event to Mixpanel:", err)
			return
		}
	}
}

func stripeSecret(stripeApiKey string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// curl https://api.stripe.com/v1/payment_intents -d amount=100 -d currency=eur
		// -d "payment_method_types[]"=card -u <sk_> -d "capture_method"=manual

		stripe.Key = stripeApiKey

		params := &stripe.PaymentIntentParams{
			Amount:             stripe.Int64(100),
			Currency:           stripe.String(string(stripe.CurrencyEUR)),
			PaymentMethodTypes: []*string{stripe.String(string(stripe.PaymentMethodTypeCard))},
			CaptureMethod:      stripe.String(string(stripe.PaymentIntentCaptureMethodManual)),
		}

		result, err := paymentintent.New(params)
		if err != nil {
			fmt.Println("Error getting a payment intent")
			he := HTTPError{
				ErrorMessage: err.Error(),
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(he)
			return
		}

		if result.ClientSecret == "" {
			fmt.Println("Missing response client secret")
			he := HTTPError{
				ErrorMessage: "Missing response client secret",
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(he)
			return
		}

		sr := StripeResponse{
			ClientSecret: result.ClientSecret,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(sr)
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
