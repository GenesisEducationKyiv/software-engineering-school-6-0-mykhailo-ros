package domain

type Subscription struct {
	ID          int
	Email       string
	Repo        string
	LastSeenTag string
}
