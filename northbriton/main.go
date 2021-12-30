package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

var names map[string]bool = map[string]bool{}

func main() {

	// Set routing rules
	http.HandleFunc("/reserve/", Reserve)
	http.HandleFunc("/list", List)

	//Use the default DefaultServeMux.
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func List(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	for name := range names {
		fmt.Fprintln(w, name)
	}
}

func Reserve(w http.ResponseWriter, r *http.Request) {
	fmt.Println()
	if r.Method != http.MethodPut {
		http.Error(w, "only puts accepted", http.StatusBadRequest)
		return
	}
	name := strings.TrimLeft(r.URL.Path, "/reserve/") //use http.StripPrefix?
	name = strings.ToLower(name)
	if strings.Contains(name, ".") {
		http.Error(w, "no sub sub domains", http.StatusBadRequest)
		return
	}

	_, err := url.ParseRequestURI(fmt.Sprintf("http://%s.northbriton.net", name))
	if err != nil {
		http.Error(w, "invalid host name "+name, http.StatusBadRequest)
		return
	}

	if names[name] {
		http.Error(w, "Already created "+name, http.StatusConflict)
		return
	}

	names[name] = true
	w.WriteHeader(http.StatusCreated)
}
