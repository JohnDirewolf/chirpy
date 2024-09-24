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

/* //Used for api hit reporting, currently disabled for the admin reporting
func (cfg *apiConfig) handler(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	response.Write([]byte(fmt.Sprintf("Hits: %v", cfg.fileserverHits.Load())))
}
*/

func (cfg *apiConfig) adminHandler(response http.ResponseWriter, request *http.Request) {
	response.Header().Add("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	webpage := fmt.Sprintf(`
	<html>
	<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
	</body>
	</html>`,
		cfg.fileserverHits.Load())
	response.Write([]byte(webpage))
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
	//File Server
	mux.Handle("/app/", http.StripPrefix("/app", hits.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	//Check on status
	mux.HandleFunc("GET /api/healthz", endHandler)
	//Hit Metric functions
	//mux.HandleFunc("GET /api/metrics", hits.handler)
	mux.HandleFunc("GET /admin/metrics", hits.adminHandler)
	mux.HandleFunc("POST /admin/reset", hits.reset)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		fmt.Printf("HTTP server ListenAndServe: %v\n", err)
	}
}
