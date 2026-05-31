package helper

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"

	"github.com/olelishna/go-shop-diploma/internal/model"
)

func SendJSONError(res http.ResponseWriter, message string, status int) {
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(status)
	json.NewEncoder(res).Encode(model.ErrorResponse{Error: message})
}

func LuhnValid(number string) bool {
	var (
		sum       int
		alternate bool
	)

	for i := len(number) - 1; i >= 0; i-- {
		n := int(number[i] - '0')
		if n < 0 || n > 9 {
			return false
		}

		if alternate {
			n *= 2
			if n > 9 {
				n = n%10 + 1
			}
		}

		sum += n
		alternate = !alternate
	}

	return sum%10 == 0
}

func IsValidOrderNumber(number string) bool {
	number = strings.TrimSpace(number)
	if number == "" {
		return false
	}

	for _, ch := range number {
		if !unicode.IsDigit(ch) {
			return false
		}
	}

	return LuhnValid(number)
}
