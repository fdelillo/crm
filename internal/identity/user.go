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

// userTransitions is the single source of truth for plan §4.6. Reactivation's
// password-dependent destination is resolved after looking up its allowed transition.
var userTransitions = map[string]map[userAction]string{
	"invited":  {reinvite: "invited", acceptInvitation: "active", changeRole: "invited", deactivate: "disabled"},
	"active":   {changeRole: "active", deactivate: "disabled"},
	"disabled": {reactivate: "active"},
}

func nextUserStatus(status string, action userAction, hasPassword bool) (string, error) {
	next, ok := userTransitions[status][action]
	if !ok {
		return "", ErrInvalidTransition
	}
	if action == reactivate && !hasPassword {
		return "invited", nil
	}
	return next, nil
}
