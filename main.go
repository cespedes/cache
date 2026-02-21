package main

import (
	"cmp"
	"log"
	"net/http"
	"os"

	"github.com/cespedes/cache/db"
	"github.com/cespedes/cache/handlers"
)

func main() {
	if err := db.Connect(); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	mux := http.NewServeMux()

	// HTML
	mux.Handle("/", http.FileServer(http.Dir("static")))

	// Location routes
	mux.HandleFunc("GET /locations", handlers.ListLocations)
	mux.HandleFunc("GET /locations/{id}", handlers.GetLocation)
	mux.HandleFunc("POST /locations", handlers.CreateLocation)
	mux.HandleFunc("PUT /locations/{id}", handlers.UpdateLocation)
	mux.HandleFunc("DELETE /locations/{id}", handlers.DeleteLocation)

	// Item routes
	mux.HandleFunc("GET /items", handlers.ListItems)
	mux.HandleFunc("GET /items/{id}", handlers.GetItem)
	mux.HandleFunc("POST /items", handlers.CreateItem)
	mux.HandleFunc("PUT /items/{id}", handlers.UpdateItem)
	mux.HandleFunc("DELETE /items/{id}", handlers.DeleteItem)

	listenAddr := cmp.Or(os.Getenv("LISTEN_ADDR"), "127.0.0.1:19970")
	log.Println("listening on " + listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}
