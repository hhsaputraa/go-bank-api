package users

import (
	"go-bank-api/utils"
	"log"
	"net/http"
)

func HandleGetAllUsersOracle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Hanya GET yang diizinkan")
		return
	}

	data, status, err := GetAllUsersOracleUsecase()
	if err != nil {
		log.Println("[modules][users][controller][GetAllUsersOracle] error:", err)
		utils.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"status":  400,
			"data":    nil,
			"message": "Failed to fetch users: " + err.Error(),
		})
		return
	}

	if !status {
		utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"status":  200,
			"data":    nil,
			"message": "No users available",
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"status":  200,
		"data":    data,
		"message": "Success Get All Users",
	})
}
