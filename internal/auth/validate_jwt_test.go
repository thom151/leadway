package auth

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
)

func TestValidateJWT(t *testing.T) {
	secret := "JijYM8yaY7DNsbdrk/WJ4mueimHvBxkCNvHyMHzmq7SJec0SUGoFir3iwZWzETXL5YNhtCXeBPxgWnuINZchjg=="
	userID := uuid.New()
	token, _ := MakeJWT(userID, secret, time.Hour)

	validateJWTtests := map[string]struct {
		token   string
		secret  string
		wantID  uuid.UUID
		wantErr bool
	}{
		"valid token":   {token, secret, userID, false},
		"invalid token": {"foo", secret, uuid.Nil, true},
		"wrong secret":  {token, "wrong", uuid.Nil, true},
	}

	for name, tt := range validateJWTtests {
		t.Run(name, func(t *testing.T) {
			got, err := ValidateJWT(tt.token, tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("expected: %#v, got: %#v", tt.wantErr, err)
			}

			if !cmp.Equal(got, tt.wantID) {
				t.Errorf("expected: %#v, got: %#v", tt.wantID, got)
			}
		})
	}
}
