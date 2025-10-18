package server

import (
	"encoding/json"
	"log"
	"net/http"
)

type HdaHandlerT func(w http.ResponseWriter, request *http.Request) error

func HdaHandlerWithJsonFallback(hdaHandler HdaHandlerT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := hdaHandler(w, r)
		if err != nil {
			log.Printf("Error: %s", err.Error())
			if clientErr, ok := err.(*ClientErr); ok {
				JsonHandler(w, clientErr.HttpCode, clientErr)
			} else {
				JsonHandler(w, http.StatusInternalServerError,
					InternalErr{
						HttpCode: http.StatusInternalServerError,
						Message:  "internal server error",
					},
				)
			}
		}
	}
}

func JsonHandler(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	jsonPay, err := json.Marshal(payload)

	if err != nil {
		log.Printf("Error when marshaling JSON: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(code)
	w.Write(jsonPay)
}
