package controllers

import (
	utils "go-bank-api/utils"
	"net/http"
)

func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	utils.WriteJSON(w, http.StatusOK, map[string]string{"status": "API is up and running!"})
}
