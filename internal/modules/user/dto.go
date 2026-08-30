package user

// UserSummary is the list/form projection of a user.
type UserSummary struct {
	ID       uint
	Name     string
	Email    string
	Phone    string
	UserType string
}
