package projects

import (
	"fmt"
	"strings"
)

// Item is a normalized project item (issue, pull request, or draft).
type Item struct {
	ID     string            `json:"id"`     // ProjectV2Item node id (PVTI_...)
	Type   string            `json:"type"`   // ISSUE, PULL_REQUEST, DRAFT_ISSUE
	Number int               `json:"number"` // issue/PR number; 0 for drafts
	Title  string            `json:"title"`
	URL    string            `json:"url"`
	State  string            `json:"state"` // OPEN, CLOSED, MERGED (empty for drafts)
	Repo   string            `json:"repo"`  // owner/name (empty for drafts)
	Body   string            `json:"body"`
	Labels []string          `json:"labels"`
	Fields map[string]string `json:"fields"` // field name -> display value (includes "Status")

	// ContentID is the node id of the underlying issue/PR/draft, used for body
	// edits. For drafts this is the DraftIssue id.
	ContentID string `json:"contentId"`
}

// Status returns the item's Status field value (its column), or "".
func (i Item) Status() string { return i.Fields["Status"] }

// IsDraft reports whether the item is a draft issue.
func (i Item) IsDraft() bool { return i.Type == "DRAFT_ISSUE" }

const itemGraphQL = `
  id type
  content{
    __typename
    ... on DraftIssue { id title body }
    ... on Issue {
      id number title url state body
      repository{ nameWithOwner }
      labels(first:20){ nodes{ name } }
    }
    ... on PullRequest {
      id number title url state body
      repository{ nameWithOwner }
      labels(first:20){ nodes{ name } }
    }
  }
  fieldValues(first:30){
    nodes{
      __typename
      ... on ProjectV2ItemFieldSingleSelectValue { name field{ ... on ProjectV2FieldCommon{ name } } }
      ... on ProjectV2ItemFieldTextValue       { text   field{ ... on ProjectV2FieldCommon{ name } } }
      ... on ProjectV2ItemFieldNumberValue     { number field{ ... on ProjectV2FieldCommon{ name } } }
      ... on ProjectV2ItemFieldDateValue       { date   field{ ... on ProjectV2FieldCommon{ name } } }
      ... on ProjectV2ItemFieldIterationValue  { title  field{ ... on ProjectV2FieldCommon{ name } } }
    }
  }`

type rawItem struct {
	ID      string
	Type    string
	Content struct {
		Typename   string `json:"__typename"`
		ID         string
		Number     int
		Title      string
		URL        string
		State      string
		Body       string
		Repository struct{ NameWithOwner string }
		Labels     struct {
			Nodes []struct{ Name string }
		}
	}
	FieldValues struct {
		Nodes []rawFieldValue
	}
}

type rawFieldValue struct {
	Typename string  `json:"__typename"`
	Name     string  // single-select
	Text     string  // text
	Number   float64 // number
	Date     string  // date
	Title    string  // iteration
	Field    struct{ Name string }
}

func (rv rawFieldValue) display() string {
	switch rv.Typename {
	case "ProjectV2ItemFieldSingleSelectValue":
		return rv.Name
	case "ProjectV2ItemFieldTextValue":
		return rv.Text
	case "ProjectV2ItemFieldNumberValue":
		return strings.TrimSuffix(fmt.Sprintf("%.4f", rv.Number), ".0000")
	case "ProjectV2ItemFieldDateValue":
		return rv.Date
	case "ProjectV2ItemFieldIterationValue":
		return rv.Title
	default:
		return ""
	}
}

func (ri rawItem) toItem() Item {
	it := Item{
		ID:        ri.ID,
		Type:      ri.Type,
		Number:    ri.Content.Number,
		Title:     ri.Content.Title,
		URL:       ri.Content.URL,
		State:     ri.Content.State,
		Repo:      ri.Content.Repository.NameWithOwner,
		Body:      ri.Content.Body,
		ContentID: ri.Content.ID,
		Fields:    map[string]string{},
	}
	for _, l := range ri.Content.Labels.Nodes {
		it.Labels = append(it.Labels, l.Name)
	}
	for _, fv := range ri.FieldValues.Nodes {
		if fv.Field.Name == "" {
			continue
		}
		if v := fv.display(); v != "" {
			it.Fields[fv.Field.Name] = v
		}
	}
	return it
}

// Items returns all items in the project (paginated).
func (p *Project) Items() ([]Item, error) {
	kind, err := p.c.resolveOwnerKind(p.Owner)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`query($login:String!, $number:Int!, $cursor:String){
      %s(login:$login){ projectV2(number:$number){
        items(first:100, after:$cursor){
          pageInfo{ hasNextPage endCursor }
          nodes{ %s }
        }
      }}
    }`, kind, itemGraphQL)

	type ownerNode struct {
		ProjectV2 struct {
			Items struct {
				PageInfo struct {
					HasNextPage bool
					EndCursor   string
				}
				Nodes []rawItem
			}
		}
	}

	var out []Item
	var cursor *string
	for {
		var resp struct {
			User         *ownerNode `json:"user"`
			Organization *ownerNode `json:"organization"`
		}
		vars := map[string]any{"login": p.Owner, "number": p.Number, "cursor": cursor}
		if err := p.c.do(query, vars, &resp); err != nil {
			return nil, err
		}
		node := resp.User
		if node == nil {
			node = resp.Organization
		}
		if node == nil {
			break
		}
		for _, ri := range node.ProjectV2.Items.Nodes {
			out = append(out, ri.toItem())
		}
		if !node.ProjectV2.Items.PageInfo.HasNextPage {
			break
		}
		c := node.ProjectV2.Items.PageInfo.EndCursor
		cursor = &c
	}
	return out, nil
}

// ItemByNumber finds an item backed by the issue/PR with the given number.
func (p *Project) ItemByNumber(number int) (Item, error) {
	items, err := p.Items()
	if err != nil {
		return Item{}, err
	}
	for _, it := range items {
		if it.Number == number {
			return it, nil
		}
	}
	return Item{}, fmt.Errorf("no item for issue/PR #%d in project #%d", number, p.Number)
}

// LinkedRepos returns the repositories the project is linked to.
func (p *Project) LinkedRepos() ([]string, error) {
	kind, err := p.c.resolveOwnerKind(p.Owner)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`query($login:String!, $number:Int!){
      %s(login:$login){ projectV2(number:$number){
        repositories(first:50){ nodes{ nameWithOwner } }
      }}
    }`, kind)
	type ownerNode struct {
		ProjectV2 struct {
			Repositories struct {
				Nodes []struct{ NameWithOwner string }
			}
		}
	}
	var resp struct {
		User         *ownerNode `json:"user"`
		Organization *ownerNode `json:"organization"`
	}
	vars := map[string]any{"login": p.Owner, "number": p.Number}
	if err := p.c.do(query, vars, &resp); err != nil {
		return nil, err
	}
	node := resp.User
	if node == nil {
		node = resp.Organization
	}
	if node == nil {
		return nil, nil
	}
	var repos []string
	for _, r := range node.ProjectV2.Repositories.Nodes {
		repos = append(repos, r.NameWithOwner)
	}
	return repos, nil
}
