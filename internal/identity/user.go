package identity

import "errors"

var (
	ErrLastAdmin         = errors.New("identity: last active admin")
	ErrInvalidTransition = errors.New("identity: invalid status transition")
	ErrUserNotFound      = errors.New("identity: user not found")
)

type userAction string

const (
	reinvite         userAction = "reinvite"
	acceptInvitation userAction = "accept"
	changeRole       userAction = "change_role"
	deactivate       userAction = "deactivate"
	reactivate       userAction = "reactivate"
)

func nextUserStatus(status string, action userAction, hasPassword bool) (string, error) {
	return "", ErrInvalidTransition
}
