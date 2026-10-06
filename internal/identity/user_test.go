package identity

import (
	"errors"
	"testing"
)

func TestUserTransitions(t *testing.T) {
	allowed := map[string]map[userAction]string{
		"invited":  {reinvite: "invited", acceptInvitation: "active", changeRole: "invited", deactivate: "disabled"},
		"active":   {changeRole: "active", deactivate: "disabled"},
		"disabled": {reactivate: "active"},
	}
	for _, status := range []string{"invited", "active", "disabled", "unknown"} {
		for _, action := range []userAction{reinvite, acceptInvitation, changeRole, deactivate, reactivate, "unknown"} {
			for _, hasPassword := range []bool{false, true} {
				t.Run(status+"/"+string(action)+"/"+map[bool]string{false: "without_password", true: "with_password"}[hasPassword], func(t *testing.T) {
					want := allowed[status][action]
					if status == "disabled" && action == reactivate && !hasPassword {
						want = "invited"
					}
					got, err := nextUserStatus(status, action, hasPassword)
					if want == "" {
						if !errors.Is(err, ErrInvalidTransition) {
							t.Fatalf("got %q, %v", got, err)
						}
					} else if err != nil || got != want {
						t.Fatalf("got %q, %v; want %q", got, err, want)
					}
				})
			}
		}
	}
}
