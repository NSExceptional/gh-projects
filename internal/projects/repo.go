package projects

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ParseRepo splits an "owner/name" string.
func ParseRepo(s string) (owner, name string, err error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo %q, want owner/name", s)
	}
	return parts[0], parts[1], nil
}

// Repo holds the resolved identifiers for a repository.
type Repo struct {
	ID            string
	NameWithOwner string
}

// Repo resolves a repository's node id from "owner/name".
func (c *Client) Repo(ownerName string) (Repo, error) {
	owner, name, err := ParseRepo(ownerName)
	if err != nil {
		return Repo{}, err
	}
	var resp struct {
		Repository *struct {
			ID            string
			NameWithOwner string
		} `json:"repository"`
	}
	vars := map[string]any{"owner": owner, "name": name}
	q := `query($owner:String!,$name:String!){ repository(owner:$owner,name:$name){ id nameWithOwner } }`
	if err := c.do(q, vars, &resp); err != nil {
		return Repo{}, err
	}
	if resp.Repository == nil {
		return Repo{}, fmt.Errorf("repository %q not found", ownerName)
	}
	return Repo{ID: resp.Repository.ID, NameWithOwner: resp.Repository.NameWithOwner}, nil
}

// LabelIDs resolves label names to their node ids within a repository,
// erroring if any name is unknown.
func (c *Client) LabelIDs(ownerName string, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	owner, name, err := ParseRepo(ownerName)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Repository *struct {
			Labels struct {
				Nodes []struct {
					ID   string
					Name string
				}
			}
		} `json:"repository"`
	}
	vars := map[string]any{"owner": owner, "name": name}
	q := `query($owner:String!,$name:String!){ repository(owner:$owner,name:$name){ labels(first:100){ nodes{ id name } } } }`
	if err := c.do(q, vars, &resp); err != nil {
		return nil, err
	}
	if resp.Repository == nil {
		return nil, fmt.Errorf("repository %q not found", ownerName)
	}
	byName := map[string]string{}
	for _, l := range resp.Repository.Labels.Nodes {
		byName[strings.ToLower(l.Name)] = l.ID
	}
	var ids []string
	for _, n := range names {
		id, ok := byName[strings.ToLower(n)]
		if !ok {
			return nil, fmt.Errorf("label %q not found in %s", n, ownerName)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

var issueURLRe = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/(?:issues|pull)/(\d+)`)

// Content is a resolved issue or pull request.
type Content struct {
	ID     string
	Number int
	Title  string
	URL    string
}

// Content resolves an issue or PR node id from either a URL or, when repo is
// given, a bare number. ref may be "123", "#123", or a full github.com URL.
func (c *Client) Content(repo, ref string) (Content, error) {
	var owner, name string
	var number int

	if m := issueURLRe.FindStringSubmatch(ref); m != nil {
		owner, name = m[1], m[2]
		number, _ = strconv.Atoi(m[3])
	} else {
		n, err := strconv.Atoi(strings.TrimPrefix(ref, "#"))
		if err != nil {
			return Content{}, fmt.Errorf("invalid issue reference %q (want a number or URL)", ref)
		}
		if repo == "" {
			return Content{}, fmt.Errorf("a bare number needs --repo owner/name")
		}
		number = n
		owner, name, err = ParseRepo(repo)
		if err != nil {
			return Content{}, err
		}
	}

	var resp struct {
		Repository *struct {
			// Issue and PullRequest inline fragments both select the same
			// fields, so they flatten into one struct.
			IssueOrPullRequest *struct {
				ID     string
				Number int
				Title  string
				URL    string
			} `json:"issueOrPullRequest"`
		} `json:"repository"`
	}
	q := `query($owner:String!,$name:String!,$number:Int!){
      repository(owner:$owner,name:$name){
        issueOrPullRequest(number:$number){
          __typename
          ... on Issue { id number title url }
          ... on PullRequest { id number title url }
        }
      }
    }`
	vars := map[string]any{"owner": owner, "name": name, "number": number}
	if err := c.do(q, vars, &resp); err != nil {
		return Content{}, err
	}
	if resp.Repository == nil || resp.Repository.IssueOrPullRequest == nil || resp.Repository.IssueOrPullRequest.ID == "" {
		return Content{}, fmt.Errorf("issue/PR #%d not found in %s/%s", number, owner, name)
	}
	r := resp.Repository.IssueOrPullRequest
	return Content{ID: r.ID, Number: r.Number, Title: r.Title, URL: r.URL}, nil
}
