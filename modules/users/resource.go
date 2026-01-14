package users

import (
	"log"
)

func GetAllUsersOracleUsecase() ([]UserResponseModel, bool, error) {
	data, status, err := GetAllUsersOracleDB()
	if !status {
		return data, status, err
	}

	if err != nil {
		return data, false, err
	}

	if len(data) == 0 {
		log.Println("[modules][users][resource][GetAllUsersOracleUsecase] User list is empty")
		return data, true, nil
	}

	return data, status, nil
}
