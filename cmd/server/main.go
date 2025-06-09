package main

import (
	"fmt"
	"log"
	customcors "mediaserver/cmd/config"
	"mediaserver/signaling"
	"mediaserver/utils/dotenv"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	port := dotenv.GetDotEnv("APP_PORT")

	r := mux.NewRouter()
	r.HandleFunc("/ws/media", signaling.HandlerConnection)

	httpHandler := customcors.SetupCors().Handler(r)

	fmt.Printf("Starting server on %s\n", port)
	log.Printf("Server start!")
	// log.Fatal(http.ListenAndServe(":"+port, httpHandler))
	err := http.ListenAndServeTLS(":"+port, "cert.pem", "key.pem", httpHandler)
	if err != nil {
		log.Fatalf("HTTPS server failed to start: %v", err)
	}
}
