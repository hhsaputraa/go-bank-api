package users

import "time"

type UserResponseModel struct {
	Id_app_users   int64      `json:"id_app_users"`
	Username       string     `json:"username"`
	Full_name      string     `json:"full_name"`
	Email          *string    `json:"email"`
	Is_admin       bool       `json:"is_admin"`
	Is_active      bool       `json:"is_active"`
	Account_status int        `json:"account_status"`
	Otp_Code       *string    `json:"otp_code"`
	Otp_Expired_at *time.Time `json:"otp_expired_at"`
	Last_login_at  time.Time  `json:"last_login_at"`
}
