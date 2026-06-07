// Package projects wraps the GitHub ProjectsV2 GraphQL API with helpers tuned
// for scripting and agent use: resolve owners/projects/fields/items by friendly
// names and numbers, and perform the common read and write operations.
package projects

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Client is a thin wrapper over go-gh's GraphQL client. It reuses gh's auth, so
// callers never handle tokens directly.
type Client struct {
	gql *api.GraphQLClient
}

// New returns a Client backed by the host gh's authentication.
func New() (*Client, error) {
	gql, err := api.NewGraphQLClient(api.ClientOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating GraphQL client (is gh authenticated?): %w", err)
	}
	return &Client{gql: gql}, nil
}

// do runs a raw GraphQL document and unmarshals the response into out.
func (c *Client) do(query string, vars map[string]any, out any) error {
	return c.gql.Do(query, vars, out)
}

// Viewer returns the login of the authenticated user.
func (c *Client) Viewer() (string, error) {
	var resp struct {
		Viewer struct{ Login string }
	}
	if err := c.do(`query { viewer { login } }`, nil, &resp); err != nil {
		return "", err
	}
	return resp.Viewer.Login, nil
}

// OwnerKind is "user" or "organization".
type OwnerKind string

const (
	OwnerUser OwnerKind = "user"
	OwnerOrg  OwnerKind = "organization"
)

// resolveOwnerKind determines whether a login is a user or an organization so
// the correct GraphQL root field can be used.
func (c *Client) resolveOwnerKind(owner string) (OwnerKind, error) {
	var resp struct {
		RepositoryOwner struct {
			Typename string `json:"__typename"`
		} `json:"repositoryOwner"`
	}
	vars := map[string]any{"login": owner}
	if err := c.do(`query($login:String!){ repositoryOwner(login:$login){ __typename } }`, vars, &resp); err != nil {
		return "", err
	}
	switch resp.RepositoryOwner.Typename {
	case "Organization":
		return OwnerOrg, nil
	case "User":
		return OwnerUser, nil
	default:
		return "", fmt.Errorf("unknown owner %q", owner)
	}
}
