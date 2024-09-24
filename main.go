package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(response, request)
	})
}

func endHandler(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	response.Write([]byte(http.StatusText(http.StatusOK)))
}

func (cfg *apiConfig) handler(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	response.Write([]byte(fmt.Sprintf("Hits: %v", cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) reset(response http.ResponseWriter, request *http.Request) {
	//Reset the server hits count
	cfg.fileserverHits.Store(0)
	//fmt.Println(cfg.fileserverHits.Load())
	//Send a response that status is Ok
	response.Header().Add("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	response.Write([]byte(http.StatusText(http.StatusOK)))
}

func main() {
	var srv http.Server
	hits := &apiConfig{}

	mux := http.NewServeMux()
	srv.Handler = mux
	srv.Addr = ":8080"
	//mux.Handle("/app/", http.StripPrefix("/app", hits.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	mux.Handle("/app/", http.StripPrefix("/app", hits.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	mux.HandleFunc("/metrics", hits.handler)
	mux.HandleFunc("/healthz", endHandler)
	mux.HandleFunc("/reset", hits.reset)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		fmt.Printf("HTTP server ListenAndServe: %v\n", err)
	}
}
