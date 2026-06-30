package auth

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// PublicUser is the user shape safe to return through the API.
type PublicUser struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	IsSuperAdmin bool   `json:"isSuperAdmin"`
}

func IsValidRole(role string) bool {
	return role == RoleAdmin || role == RoleUser
}
