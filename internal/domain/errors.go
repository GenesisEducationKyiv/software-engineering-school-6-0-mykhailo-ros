package domain

import "errors"

var (
	ErrNotFound          = errors.New("not found")
	ErrRepoNotFound      = errors.New("repository not found")
	ErrAlreadySubscribed = errors.New("already subscribed")
	ErrRateLimited       = errors.New("rate limited")
)
