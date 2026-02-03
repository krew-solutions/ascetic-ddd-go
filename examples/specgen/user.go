package main

//go:generate go run github.com/krew-solutions/ascetic-ddd-go/cmd/specgen -type=User

// User represents a domain user
type User struct {
	ID     int64
	Age    int
	Active bool
	Name   string
	Email  string
}

// AdultUserSpec checks if user is adult (age >= 18)
//spec:sql
func AdultUserSpec(u User) bool {
	return u.Age >= 18
}

// ActiveUserSpec checks if user is active
//spec:sql
func ActiveUserSpec(u User) bool {
	return u.Active == true
}

// ValidEmailSpec checks if user has email
//spec:sql
func ValidEmailSpec(u User) bool {
	return u.Email != ""
}

// PremiumUserSpec checks if user is premium (adult, active, and has name)
//spec:sql
func PremiumUserSpec(u User) bool {
	return u.Age >= 18 && u.Active && u.Name != ""
}

// YoungUserSpec checks if user is young (age < 25)
//spec:sql
func YoungUserSpec(u User) bool {
	return u.Age < 25
}

// InactiveUserSpec checks if user is inactive
//spec:sql
func InactiveUserSpec(u User) bool {
	return !u.Active
}
