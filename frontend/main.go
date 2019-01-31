package main

import (
	"log"
	"net/http"
)

func main() {
	fs := http.StripPrefix("/docs/", http.FileServer(http.Dir("static/site")))
	http.Handle("/docs/", fs)

	fs2 := http.FileServer(http.Dir("static/frontpage"))
	http.Handle("/", fs2)

	log.Println("Listening on :3000...")
	http.ListenAndServe(":3000", nil)
}
