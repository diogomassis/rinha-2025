package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	instanceName := os.Getenv("INSTANCE_NAME")
	if instanceName == "" {
		instanceName = "Unknown"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Requisição recebida na instância: %s\n", instanceName)
		fmt.Fprintf(w, "Hello from instance: %s", instanceName)
	})

	port := "8080"
	log.Printf("Servidor [%s] escutando na porta %s\n", instanceName, port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
