package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
)

var names map[string]bool = map[string]bool{}

func main() {

	api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
	if err != nil {
		log.Fatalf("can't getcloudflare client %s", err)
	}
	nbid, err := api.ZoneIDByName("northbriton.net")
	if err != nil {
		log.Fatalf("can't get details %s", err)
	}
	fmt.Printf("trying to get records for %s\n", nbid)
	records, err := api.DNSRecords(context.TODO(), nbid, cloudflare.DNSRecord{}) //Type: "TXT"
	if err != nil {
		log.Fatalf("can't get dns records %s", err)
	}

	for _, record := range records {
		fmt.Printf(record.Name)
	}
	// Set routing rules
	http.HandleFunc("/reserve/", withLogging(Reserve))
	http.HandleFunc("/list", withLogging(List))

	//Use the default DefaultServeMux.
	err = http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func List(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	log.Println("list request")
	for name := range names {
		fmt.Fprintln(w, name)
	}
}

//https://github.com/gorilla/handlers
func withLogging(h http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(rw, r) // serve the original request
		log.Printf("%s %s: duration %d", r.RequestURI, r.Method, time.Since(start))
	}
}

var domainregex = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9\-]{0,61}[A-Za-z0-9])?$`)

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

	if !domainregex.MatchString(name) {
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
