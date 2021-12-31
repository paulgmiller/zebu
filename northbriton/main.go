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

var domainregex = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9\-]{0,61}[A-Za-z0-9])?$`)

const nb = "northbriton.net"

func main() {

	if names, err := listRecords(context.Background()); err != nil {
		log.Fatalf(err.Error())
	} else {
		log.Printf("Starting with %d records", len(names))
	}
	// Set routing rules
	http.HandleFunc("/reserve/", withLogging(Reserve))
	http.HandleFunc("/list", withLogging(List))

	//Use the default DefaultServeMux.
	err := http.ListenAndServe(":8000", nil)
	if err != nil {
		log.Fatal(err)
	}
}

func listRecords(ctx context.Context) (map[string]string, error) {
	api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
	if err != nil {
		return nil, fmt.Errorf("can't getcloudflare client %w", err)
	}
	nbid, err := api.ZoneIDByName(nb)
	if err != nil {
		return nil, fmt.Errorf("can't get zone %w", err)
	}
	records, err := api.DNSRecords(ctx, nbid, cloudflare.DNSRecord{Type: "TXT"})
	if err != nil {
		return nil, fmt.Errorf("can't get dns records %w", err)
	}

	result := map[string]string{}
	for _, record := range records {
		result[record.Name] = record.Content
	}
	return result, nil
}

func List(w http.ResponseWriter, r *http.Request) {
	names, err := listRecords(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	for name, value := range names {
		fmt.Fprintln(w, name+"->"+value)
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

func Reserve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "only puts accepted", http.StatusBadRequest)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/reserve/") //use http.StripPrefix?
	name = strings.ToLower(name)
	if strings.Contains(name, ".") {
		http.Error(w, "no sub sub domains", http.StatusBadRequest)
		return
	}

	if !domainregex.MatchString(name) {
		http.Error(w, "invalid host name "+name, http.StatusBadRequest)
		return
	}

	names, err := listRecords(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fullname := "_dnslink." + name + "." + nb
	if content, found := names[fullname]; found {
		http.Error(w, fmt.Sprintf("Already created %s as %s", name, content), http.StatusConflict)
		return
	}

	api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
	if err != nil {
		http.Error(w, fmt.Errorf("can't getcloudflare client %w", err).Error(), http.StatusInternalServerError)
		return
	}
	nbid, err := api.ZoneIDByName(nb)
	if err != nil {
		http.Error(w, fmt.Errorf("can't get zone %w", err).Error(), http.StatusInternalServerError)
		return

	}
	resp, err := api.CreateDNSRecord(r.Context(), nbid, cloudflare.DNSRecord{
		Name:    fullname,
		Type:    "TXT",
		Content: "dnslink=/ipfs/bafybeieenxnjdjm7vbr5zdwemaun4sw4iy7h4imlvvl433q6gzjg6awdpq",
	})
	if err != nil || !resp.Success {
		http.Error(w, "Error reserving "+name, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
