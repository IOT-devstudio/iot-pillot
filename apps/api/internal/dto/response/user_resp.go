package response

type LoginResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	UserID       int    `json:"user_id"`
}
