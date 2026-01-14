package users

import (
	"context"
	"fmt"
	"go-bank-api/constants"
	"go-bank-api/database"
	"go-bank-api/utils"
	"log"
	"time"
)

func GetAllUsersOracleDB() ([]UserResponseModel, bool, error) {
	if database.DbInstance == nil {
		return nil, false, fmt.Errorf("database belum terkoneksi")
	}

	var datas []UserResponseModel

	query := `
		SELECT id_app_users, username, full_name, email, is_admin, is_active, account_status, last_login_at, otp_code, otp_expired_at
		FROM app_users
		ORDER BY id_app_users ASC
	`

	rows, err := database.DbInstance.QueryContext(context.Background(), query)
	if err != nil {
		log.Println("[modules][users][resource_db][GetAllUsersOracleDB] error on query", err.Error())
		return datas, false, err
	}
	defer rows.Close()

	for rows.Next() {
		var u UserResponseModel
		var isAdminInt, isActiveInt, accountStatus int
		var lastLoginRaw interface{}
		var otpCode interface{}
		var otpExpiredAt interface{}

		err := rows.Scan(
			&u.Id_app_users, &u.Username, &u.Full_name, &u.Email,
			&isAdminInt, &isActiveInt, &accountStatus, &lastLoginRaw,
			&otpCode, &otpExpiredAt,
		)
		if err != nil {
			log.Printf("[modules][users][resource_db][GetAllUsersOracleDB] Warning scan user: %v", err)
			continue
		}

		u.Is_admin = (utils.InterfaceToInt(isAdminInt) == constants.AdminRoleValue)
		u.Is_active = (utils.InterfaceToInt(isActiveInt) == constants.ActiveUserStatus)
		u.Account_status = accountStatus

		if lastLoginRaw != nil {
			if t, ok := lastLoginRaw.(time.Time); ok {
				u.Last_login_at = t
			}
		}

		if otpCode != nil {
			if str, ok := otpCode.(string); ok {
				u.Otp_Code = &str
			}
		}

		if otpExpiredAt != nil {
			if t, ok := otpExpiredAt.(time.Time); ok {
				u.Otp_Expired_at = &t
			}
		}

		datas = append(datas, u)
	}

	return datas, true, nil
}
