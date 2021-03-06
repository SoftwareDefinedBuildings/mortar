package main

import (
	"log"
	"net/http"
)

func main() {
	fs := http.StripPrefix("/docs/", http.FileServer(http.Dir("static/site")))
	http.Handle("/docs/", fs)

	fs2 := http.FileServer(http.Dir("static/landing"))
	http.Handle("/", fs2)

	fs3 := http.StripPrefix("/login/", http.FileServer(http.Dir("static/login")))
	http.Handle("/login/", fs3)

	fs4 := http.StripPrefix("/pymortar/", http.FileServer(http.Dir("static/pydocs")))
	http.Handle("/pymortar/", fs4)

	log.Println("Listening on :3000...")
	log.Fatal(http.ListenAndServe(":3000", nil))
}
