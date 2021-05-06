package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// Handler is the entry point for this fission function
func Handler(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body",
			http.StatusInternalServerError)
	}
	results := string(body)
	fmt.Println(results)
	w.Write([]byte("Hello " + results))
}
