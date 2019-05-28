package main

import (
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
)

func main() {
	router := httprouter.New()
	router.NotFound = http.FileServer(http.Dir("public"))
	log.Fatal(http.ListenAndServe(":6001", router))
}
