package main

import (
	_ "embed"
	"flag"
	"fmt"
	"net/http"
)

//go:embed static/index.html
var indexHTML string

var port = *(flag.Int("port", 3000, "Port to run the server on"))

func main() {
	url := fmt.Sprintf("http://localhost:%d", port)
	fmt.Printf("Server running at %s\n", url)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(indexHTML))
	})

	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		fmt.Printf("Error starting server: %v\n", err)
	}
}
