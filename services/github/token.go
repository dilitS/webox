//nolint:revive // Token source names follow the small interfaces they implement.
package github

import (
	"context"

	"github.com/dilitS/webox/secrets"
)

type SecretsTokenSource struct {
	Backend secrets.Backend
	Account string
}

func (s SecretsTokenSource) Token(context.Context) (string, error) {
	token, err := secrets.GetGitHubPAT(s.Backend, s.Account)
	if err != nil {
		return "", err
	}
	return string(token), nil
}
