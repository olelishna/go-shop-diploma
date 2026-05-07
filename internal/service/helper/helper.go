package helper

import (
	"encoding/json"
	"net/http"

	"github.com/olelishna/go-shop-diploma/internal/model"
)

func SendJSONError(res http.ResponseWriter, message string, status int) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(status)
	json.NewEncoder(res).Encode(model.ErrorResponse{Error: message})
}
