package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type Incoming struct {
	Msg string `json:"msg"`
}


func test(rw http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)

	var in Incoming
	err := decoder.Decode(&in)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println(in.Msg)
}
  
func main() {
    http.HandleFunc("/test", test)
    log.Fatal(http.ListenAndServe(":8080", nil))
}