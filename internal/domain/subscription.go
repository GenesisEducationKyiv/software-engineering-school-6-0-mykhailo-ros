package domain

type Subscription struct {
	ID               int
	Email            string
	Repo             string
	Confirmed        bool
	ConfirmToken     string
	UnsubscribeToken string
	LastSeenTag      string
}
