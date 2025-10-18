package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/effiware/goth-template/internal/server/models"
)

type EndpointHandlerT func(w http.ResponseWriter, request *http.Request) (int, any, error)

func JsonHandler(endpointHandler EndpointHandlerT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		code, payload, err := endpointHandler(w, r)
		if err != nil {
			log.Printf("Error: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		jsonPay, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Error when marshaling JSON: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(code)
		w.Write(jsonPay)
	}
}

func GetClicks(w http.ResponseWriter, r *http.Request) (int, any, error) {
	ctx := r.Context()
	_ = ctx // currently unused, but may be useful for logging or tracing in the future
	clicks := models.ClicksStore
	return http.StatusOK, models.Clicks{Count: clicks.GetCount()}, nil
}

func IncrementClicks(w http.ResponseWriter, r *http.Request) (int, any, error) {
	ctx := r.Context()
	_ = ctx // currently unused, but may be useful for logging or tracing in the future
	clicks := models.ClicksStore
	clicks.Increment()
	return http.StatusNoContent, map[string]string{}, nil
}
