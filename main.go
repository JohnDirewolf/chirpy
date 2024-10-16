package main

import (
	"encoding/json"
	"fmt"
	"strings"

	//"log"
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

func profanityFilter(rawString string) string {
	rawArray := strings.Split(rawString, " ")
	for i := 0; i < len(rawArray); i++ {
		//This could be done as a profantity array with the words to check, allowing easier adding of profanity, but as there are only three it is just a long boolean.
		if strings.ToLower(rawArray[i]) == "kerfuffle" || strings.ToLower(rawArray[i]) == "sharbert" || strings.ToLower(rawArray[i]) == "fornax" {
			rawArray[i] = "****"
		}
	}
	return strings.Join(rawArray, " ")
}

func validateChirp(response http.ResponseWriter, request *http.Request) {
	//So this has been massively rewritten as there are major changes in the structure of the responses. I commited the old version to have it available if needed.

	//This is the structure for the request which has the Chirp we want to validate.
	type requestParameters struct {
		Body string `json:"body"`
	}
	//This is the now common structure of a response.
	type responseParameters struct {
		CleanedBody string `json:"cleaned_body"`
	}

	var dataToReturn responseParameters

	decoder := json.NewDecoder(request.Body)
	params := requestParameters{}
	err := decoder.Decode(&params)
	if err != nil {
		//There was an error in the decoding, so we do a error response, we do not use params here.
		response.WriteHeader(500)
		dataToReturn = responseParameters{
			CleanedBody: "Error reported! Something went wrong",
		}
	} else {
		//Check if the length of the chirp is vailid first
		//fmt.Printf("Params Body: %v", params.Body)
		if len(params.Body) > 140 {
			response.WriteHeader(400)
			dataToReturn = responseParameters{
				CleanedBody: params.Body,
			}
		} else {
			//Tweet is valid
			response.WriteHeader(200)
			dataToReturn = responseParameters{
				CleanedBody: profanityFilter(params.Body),
			}
		}
	}
	//So we have set our header status code and have the response data now to marshal and write it.
	response.Header().Set("Content-Type", "application/json")
	//We could do some error checks on the json marshalling but it is all hard coded so it should work or it does not when we build it.
	dataMarshalled, _ := json.Marshal(dataToReturn)
	response.Write(dataMarshalled)
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
	//Chirp functions
	mux.HandleFunc("POST /api/validate_chirp", validateChirp)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		fmt.Printf("HTTP server ListenAndServe: %v\n", err)
	}
}
