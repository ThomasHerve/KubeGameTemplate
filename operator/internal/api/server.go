package api

import (
	"fmt"
	"net/http"
)


func StartHTTPServer() {

	http.HandleFunc("/hello", helloHandler)

	fmt.Println("HTTP API listening on :8080")

	err := http.ListenAndServe(":8080", nil)

	if err != nil {
		panic(err)
	}
}


func helloHandler(w http.ResponseWriter, r *http.Request) {

	fmt.Println("Received hello request")

	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte("Hello"))
}