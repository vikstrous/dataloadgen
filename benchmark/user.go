package benchmark

// User is some kind of database backed model
//
//go:generate go run github.com/vektah/dataloaden UserLoader int User
type User struct {
	ID   string
	Name string
}
