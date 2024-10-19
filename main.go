package main

import (
	//"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github/JohnDirewolf/chirpy/internal/auth"
	"github/JohnDirewolf/chirpy/internal/database"

	//"io"
	"os"
	"strings"
	"time"

	//"log"
	"net/http"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	PLATFORM       string
	SECRET         string
	POLKA          string
}

type chirpsResponse struct {
	Id        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserId    uuid.UUID `json:"user_id"`
}

type userRequest struct {
	Password string `json:"password"`
	Email    string `json:"email"`
	//ExpiresInSeconds int32  `json:"expires_in_seconds"` - No longer used, access token expire in 1 hour.
}

type userResponse struct {
	Id           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(response, request)
	})
}

func utilityProfanityFilter(rawString string) string {
	rawArray := strings.Split(rawString, " ")
	for i := 0; i < len(rawArray); i++ {
		//This could be done as a profantity array with the words to check, allowing easier adding of profanity, but as there are only three it is just a long boolean.
		if strings.ToLower(rawArray[i]) == "kerfuffle" || strings.ToLower(rawArray[i]) == "sharbert" || strings.ToLower(rawArray[i]) == "fornax" {
			rawArray[i] = "****"
		}
	}
	return strings.Join(rawArray, " ")
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
	//Check if this method can bee accessed.
	if cfg.PLATFORM != "dev" {
		fmt.Println("Reset attempted in non-dev environment")
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusForbidden)
		response.Write([]byte("Forbidden: reset only allowed in dev environment"))
		return
	}

	//Clear the users table.
	err := cfg.dbQueries.ResetUsers(context.Background())
	if err != nil {
		fmt.Printf("Error resetting the Users table: %v\n", err)
		//We do not return, we just go on to reset the server hit counts
	}

	//Reset the server hits count
	cfg.fileserverHits.Store(0)

	//fmt.Println(cfg.fileserverHits.Load())
	//Send a response that status is Ok
	response.Header().Add("Content-Type", "text/plain; charset=utf-8")
	response.WriteHeader(http.StatusOK)
	response.Write([]byte(http.StatusText(http.StatusOK)))
}

func (cfg *apiConfig) handlerPostChirps(response http.ResponseWriter, request *http.Request) {
	//This is the structure for the request which has the Chirp we want to validate.

	//This is a raw JSON check as I seem to be getting bad information from Boot.dev.
	/*
		bodyBytes, requestErr := io.ReadAll(request.Body)
		if requestErr != nil {
			// handle the error
			fmt.Println("Error in ReadAll")
		}

		// Convert bytes to a string to see the raw JSON
		rawJSON := string(bodyBytes)

		// Log or print the raw JSON
		fmt.Println("Raw JSON:", rawJSON)

		// Reset the request body if you need to parse it again later
		request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	*/

	type requestParameters struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	decoder := json.NewDecoder(request.Body)
	requestParams := requestParameters{}
	err := decoder.Decode(&requestParams)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Could not decode JSON in request."))
		return
	}

	//fmt.Printf("Post Chirps: Body: %v\n", requestParams.Body)
	//fmt.Printf("Post Chirps: UserID: %v\n", requestParams.UserID)

	//Validate credentials sent.
	//Get the user token from the request.
	userToken, err := auth.GetBearerToken(request.Header)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Unathorized: Please login first."))
		return
	}

	requestParams.UserID, err = auth.ValidateJWT(userToken, cfg.SECRET)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Unathorized: credentials invalid. Please login again."))
		return
	}

	//Check if the length of the chirp is vailid first
	if len(requestParams.Body) > 140 {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte("Bad Request: Chirp is longer then 140 characters."))
		return
	}

	//Apply our Profanity Filter
	requestParams.Body = utilityProfanityFilter(requestParams.Body)

	//Valid Tweet, save to chirps table.
	//While the CreateChirpParams and my requestParams are similar in structure, per Boots suggestion it is best to do an explicit copy betwen structures.
	createChirpParams := database.CreateChirpParams{
		Body:   requestParams.Body,
		UserID: requestParams.UserID,
	}

	returnChirpParams, err := cfg.dbQueries.CreateChirp(context.Background(), createChirpParams)
	if err != nil {
		//fmt.Printf("CreateChirp error: %v\n", err)
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Could not save Chirp."))
		return
	}

	//Again, we are doing a explicit copy to our response struct from the response from the query.
	responseParams := chirpsResponse{
		Id:        returnChirpParams.ID,
		CreatedAt: returnChirpParams.CreatedAt,
		UpdatedAt: returnChirpParams.UpdatedAt,
		Body:      returnChirpParams.Body,
		UserId:    returnChirpParams.UserID,
	}

	dataMarshalled, err := json.Marshal(responseParams)
	if err != nil {
		//There was an error in the decoding, so we do a error response, we do not use params here.
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Could not create response."))
		return
	}

	//Success
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusCreated)
	response.Write(dataMarshalled)
}

func (cfg *apiConfig) handlerGetChirps(response http.ResponseWriter, request *http.Request) {
	var chirpList []database.Chirp
	var uuidUserID uuid.UUID
	var sortOrder string
	var err error
	const desc string = "desc"

	//Check if we are getting all chirps or just for single user.
	userID := request.URL.Query().Get("author_id")
	//A blank or non-existent author_id causes an error. So skipping over the Parsing if needed.
	uuidUserID, err = uuid.Parse(userID)
	if err != nil {
		//Length 0 is not an error, that just means we have no author_id.
		if err.Error() != "invalid UUID length: 0" {
			response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			response.WriteHeader(http.StatusBadRequest)
			response.Write([]byte("Bad Request: user id is malformed."))
			return
		}
	}

	//Check if we want to sort ASC (default) or Desc
	sortOrder = strings.ToLower(request.URL.Query().Get("sort"))
	//Only desc does anything, missing or bad data just defaults to asc in the if-then below.

	//Use either the query for all chirps or the query for just the chirps of the user id
	if userID == "" {
		if sortOrder == desc {
			chirpList, err = cfg.dbQueries.GetAllChirpsDesc(context.Background())
		} else {
			chirpList, err = cfg.dbQueries.GetAllChirpsAsc(context.Background())
		}
	} else {
		if sortOrder == desc {
			chirpList, err = cfg.dbQueries.GetChirpsByUserIDdesc(context.Background(), uuidUserID)
		} else {
			chirpList, err = cfg.dbQueries.GetChirpsByUserIDasc(context.Background(), uuidUserID)
		}
	}

	//Check for an error in the query
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Could not retrieve Chirps."))
		return
	}

	//Check if we have no tweets to show. This also works for a user id that does not exist.
	if len(chirpList) == 0 {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusNotFound)
		response.Write([]byte("Not Found: No Chirps found."))
		return
	}

	//Go through the return chripList and convert it to our JSON structure
	chripListResponse := make([]chirpsResponse, 0, len(chirpList))
	for i := 0; i < len(chirpList); i++ {
		chripListResponse = append(chripListResponse, chirpsResponse{
			Id:        chirpList[i].ID,
			CreatedAt: chirpList[i].CreatedAt,
			UpdatedAt: chirpList[i].UpdatedAt,
			Body:      chirpList[i].Body,
			UserId:    chirpList[i].UserID,
		})
	}

	//chripListResponse should now be an array of JSON compatible items. Marshall and send
	chripListMarshalled, err := json.Marshal(chripListResponse)
	if err != nil {
		//There was an error in the decoding, so we do a error response, we do not use params here.
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Could not process Chirps to JSON."))
		return
	}

	//Success
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	response.Write(chripListMarshalled)
}

func (cfg *apiConfig) handlerGetChirpByID(response http.ResponseWriter, request *http.Request) {

	//Extract the Chirp ID
	chirpID, err := uuid.Parse(request.PathValue("chirpID"))
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte("Error! Invalid Chirp ID."))
		return
	}

	chirp, err := cfg.dbQueries.GetChirpByID(context.Background(), chirpID) //Chirp, error
	if err != nil {
		if err == sql.ErrNoRows {
			// No chirp found with this ID
			response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			response.WriteHeader(http.StatusNotFound)
			response.Write([]byte("Error! Chirp not found."))
			return
		}
		// Some other error occurred
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Error! Server error fetching Chirp."))
		return
	}

	chripMarshalled, err := json.Marshal(chirpsResponse{
		Id:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserId:    chirp.UserID,
	})

	if err != nil {
		//There was an error in the decoding, so we do an error response, we do not use params here.
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Could not process Chirp to JSON."))
		return
	}

	//Success
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	response.Write(chripMarshalled)
}

func (cfg *apiConfig) handlerDeleteChirpByID(response http.ResponseWriter, request *http.Request) {
	//Validate credentials sent.
	//Get the user token from the request.
	userToken, err := auth.GetBearerToken(request.Header)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Unathorized: Please login first."))
		return
	}

	userID, err := auth.ValidateJWT(userToken, cfg.SECRET)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Unathorized: credentials invalid. Please login again."))
		return
	}

	//We have the User ID from the token, check if that matches the chirp author.
	//Extract the Chirp ID from the URL of the request
	chirpID, err := uuid.Parse(request.PathValue("chirpID"))
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte("Error! Invalid Chirp ID."))
		return
	}

	chirp, err := cfg.dbQueries.GetChirpByID(context.Background(), chirpID) //Chirp, error
	if err != nil {
		if err == sql.ErrNoRows {
			// No chirp found with this ID
			response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
			response.WriteHeader(http.StatusNotFound)
			response.Write([]byte("Error! Chirp not found."))
			return
		}
		// Some other error occurred
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Error! Server error fetching Chirp."))
		return
	}

	//Verify the current user is the tweet to delete author.
	if chirp.UserID != userID {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusForbidden)
		response.Write([]byte("Forbidden! Only author can delete chirps."))
		return
	}

	//Delete the tweet.
	err = cfg.dbQueries.DeleteChirpByID(context.Background(), chirp.ID)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Error! Server error deleteing Chirp."))
		return
	}

	//Success
	response.WriteHeader(http.StatusNoContent)
	return
}

func (cfg *apiConfig) createUser(response http.ResponseWriter, request *http.Request) {
	//Decode the request, get the email of the user we are creating.
	decoder := json.NewDecoder(request.Body)
	requestBody := userRequest{}
	err := decoder.Decode(&requestBody)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte("Bad Request: Did not understand request."))
		return
	}

	//Hash the password.
	requestBody.Password, err = auth.HashPassword(requestBody.Password)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte("Bad Request: Problem with password."))
		return
	}

	returned, err := cfg.dbQueries.CreateUser(context.Background(), database.CreateUserParams{
		HashedPassword: requestBody.Password,
		Email:          requestBody.Email,
	})
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Failed to create new user."))
		return
	}

	//fmt.Printf("createUser returned.ID: %v\n", returned.ID)

	dataMarshalled, err := json.Marshal(userResponse{
		Id:          returned.ID,
		CreatedAt:   returned.CreatedAt,
		UpdatedAt:   returned.UpdatedAt,
		Email:       returned.Email,
		IsChirpyRed: returned.IsChirpyRed,
	})
	//fmt.Printf("createUser dataMarshalled: %v\n", dataMarshalled)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Failed create response."))
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusCreated)
	response.Write(dataMarshalled)
	return
}

func (cfg *apiConfig) handlerUpdateUser(response http.ResponseWriter, request *http.Request) {
	//Validate credentials sent.
	//Get the user token from the request.
	userToken, err := auth.GetBearerToken(request.Header)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Unathorized: Please login first."))
		return
	}

	userID, err := auth.ValidateJWT(userToken, cfg.SECRET)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Unathorized: credentials invalid. Please login again."))
		return
	}

	//We have a valid user.
	type requestParameters struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}

	decoder := json.NewDecoder(request.Body)
	requestParams := requestParameters{}
	err = decoder.Decode(&requestParams)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Could not decode JSON in request."))
		return
	}

	hashedPassword, err := auth.HashPassword(requestParams.Password)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Error processing email/password."))
		return
	}

	err = cfg.dbQueries.UpdateUser(context.Background(), database.UpdateUserParams{
		Email:          requestParams.Email,
		HashedPassword: hashedPassword,
		ID:             userID,
	})
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Error unable to update email/password."))
		return
	}

	userData, err := cfg.dbQueries.GetUser(context.Background(), requestParams.Email)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Error unable retrieve updated user data."))
		return
	}

	dataMarshalled, err := json.Marshal(userResponse{
		Id:          userData.ID,
		CreatedAt:   userData.CreatedAt,
		UpdatedAt:   userData.UpdatedAt,
		Email:       userData.Email,
		IsChirpyRed: userData.IsChirpyRed,
	})
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Failed create response."))
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	response.Write(dataMarshalled)
	return
}

func (cfg *apiConfig) handlerLogin(response http.ResponseWriter, request *http.Request) {
	//Similar to the create user, and possibly these could be combined as much is the same code.
	//Decode the request, get the email of the user we are creating.
	decoder := json.NewDecoder(request.Body)
	requestBody := userRequest{}
	err := decoder.Decode(&requestBody)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte("Bad Request: Did not understand request."))
		return
	}

	//Get the user information, including the hashed_password.
	userData, err := cfg.dbQueries.GetUser(context.Background(), requestBody.Email)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Incorrect email or password"))
		return
	}

	//Validate the password.
	err = auth.CheckPasswordHash(requestBody.Password, userData.HashedPassword)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Incorrect email or password"))
		return
	}

	//The user was verified so we now create and pass them a token.
	token, err := auth.MakeJWT(userData.ID, cfg.SECRET)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Unable to generate token"))
		return
	}

	//Try to create a refresh token, if it fails, send error.
	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Unable to generate refresh token"))
		return
	}

	//Store the refresh token in the refresh_tokens table
	err = cfg.dbQueries.CreateRefreshToken(context.Background(), database.CreateRefreshTokenParams{
		Token:     refreshToken,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().Add(time.Hour * 24 * 60).UTC(),
		UserID:    userData.ID,
	})
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Unable to store refresh token"))
		return
	}

	dataMarshalled, err := json.Marshal(userResponse{
		Id:           userData.ID,
		CreatedAt:    userData.CreatedAt,
		UpdatedAt:    userData.UpdatedAt,
		Email:        userData.Email,
		Token:        token,
		RefreshToken: refreshToken,
		IsChirpyRed:  userData.IsChirpyRed,
	})
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Failed create response."))
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	response.Write(dataMarshalled)
	return
}

func (cfg *apiConfig) handlerRefresh(response http.ResponseWriter, request *http.Request) {
	//Get the refresh token from the headers
	refreshToken, err := auth.GetBearerToken(request.Header)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Unauthorized: Please try to login again."))
		return
	}

	//Try to get the refreshToken data from the database.
	refreshTokenData, err := cfg.dbQueries.GetRefreshToken(context.Background(), refreshToken)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Unauthorized: Please try to login again."))
		return
	}

	accessToken, err := auth.MakeJWT(refreshTokenData.UserID, cfg.SECRET)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Cannot generate access token."))
		return
	}

	dataMarshalled, err := json.Marshal(map[string]string{"token": accessToken})
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Failed create response."))
		return
	}

	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(http.StatusOK)
	response.Write(dataMarshalled)
	return
}

func (cfg *apiConfig) handlerRevoke(response http.ResponseWriter, request *http.Request) {
	//Get the refresh token from the headers
	refreshToken, err := auth.GetBearerToken(request.Header)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Unauthorized: Please try to login again."))
		return
	}

	err = cfg.dbQueries.RevokeRefreshToken(context.Background(), refreshToken)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Failed to revoke token."))
		return
	}

	response.WriteHeader(http.StatusNoContent)
}

func (cfg *apiConfig) handlerUpgradeUser(response http.ResponseWriter, request *http.Request) {
	//Check if this is an authorized response from Polka
	requestKey, _ := auth.GetAPIKey(request.Header)
	if requestKey != cfg.POLKA {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusUnauthorized)
		response.Write([]byte("Unauthorized: You cannot take this action."))
		return
	}

	type polkaParameters struct {
		Event string `json:"event"`
		Data  struct {
			UserID string `json:"user_id"`
		} `json:"data"`
	}

	decoder := json.NewDecoder(request.Body)
	polkaParams := polkaParameters{}
	err := decoder.Decode(&polkaParams)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte("Bad Request: malformed request."))
		return
	}

	//Check if the event is a known type.
	if polkaParams.Event != "user.upgraded" {
		response.WriteHeader(http.StatusNoContent)
		return
	}

	//Upgrade user.
	userID, err := uuid.Parse(polkaParams.Data.UserID)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte("Bad Request: Invalid user id."))
		return
	}
	updateResult, err := cfg.dbQueries.UpgradeUser(context.Background(), userID)
	if err != nil {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte("Internal Server Error: Unable to upgrade user."))
		return
	}
	rowsReturned, err := updateResult.RowsAffected()
	if rowsReturned == 0 {
		response.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		response.WriteHeader(http.StatusNotFound)
		response.Write([]byte("Not Found: User not found."))
		return
	}

	//Success
	response.WriteHeader(http.StatusNoContent)
	return
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		fmt.Printf("Error in connecting to Database: %v", err)
		os.Exit(1)
	}

	cfg := &apiConfig{
		dbQueries: database.New(db),
		PLATFORM:  os.Getenv("PLATFORM"),
		SECRET:    os.Getenv("SECRET"),
		POLKA:     os.Getenv("POLKA_KEY"),
	}

	testing := false

	if testing {
		//Testing JWT
		fmt.Println("TESTING!")
		testUUID, _ := uuid.Parse("5ca51bf7-8e12-444e-8404-568b11f5719a")
		auth.TestJWTGood(testUUID)
		fmt.Println("")
		auth.TestJWTBad(testUUID)
	}

	var srv http.Server
	mux := http.NewServeMux()
	srv.Handler = mux
	srv.Addr = ":8080"
	//File Server
	mux.Handle("/app/", http.StripPrefix("/app", cfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))))
	//Check on status
	mux.HandleFunc("GET /api/healthz", endHandler)
	//Hit Metric functions
	//mux.HandleFunc("GET /api/metrics", hits.handler)
	mux.HandleFunc("GET /admin/metrics", cfg.adminHandler)
	mux.HandleFunc("POST /admin/reset", cfg.reset)
	//Chirp functions
	mux.HandleFunc("POST /api/chirps", cfg.handlerPostChirps)
	mux.HandleFunc("GET /api/chirps", cfg.handlerGetChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", cfg.handlerGetChirpByID)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", cfg.handlerDeleteChirpByID)
	//User functions
	mux.HandleFunc("POST /api/login", cfg.handlerLogin)
	mux.HandleFunc("POST /api/users", cfg.createUser)
	mux.HandleFunc("POST /api/refresh", cfg.handlerRefresh)
	mux.HandleFunc("POST /api/revoke", cfg.handlerRevoke)
	mux.HandleFunc("PUT /api/users", cfg.handlerUpdateUser)
	//Webhooks
	mux.HandleFunc("POST /api/polka/webhooks", cfg.handlerUpgradeUser)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		fmt.Printf("HTTP server ListenAndServe: %v\n", err)
	}
}
