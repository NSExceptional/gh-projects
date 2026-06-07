package projects

import (
	"fmt"
	"strings"
)

// Field is a project field definition. Options is populated for single-select
// fields; Iterations for iteration fields.
type Field struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	DataType   string      `json:"dataType"` // TEXT, NUMBER, DATE, SINGLE_SELECT, ITERATION, TITLE, ...
	Options    []Option    `json:"options,omitempty"`
	Iterations []Iteration `json:"iterations,omitempty"`
}

// Option is a single-select field choice. The ID is the opaque value used when
// setting the field.
type Option struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Iteration is an iteration-field cycle.
type Iteration struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// OptionByName returns the option whose name matches (case-insensitive), or an
// error listing the valid options.
func (f Field) OptionByName(name string) (Option, error) {
	for _, o := range f.Options {
		if strings.EqualFold(o.Name, name) {
			return o, nil
		}
	}
	var names []string
	for _, o := range f.Options {
		names = append(names, fmt.Sprintf("%q", o.Name))
	}
	return Option{}, fmt.Errorf("no option %q in field %q; valid: %s", name, f.Name, strings.Join(names, ", "))
}

// Project is a ProjectV2 with its field definitions resolved.
type Project struct {
	ID          string
	Number      int
	Title       string
	Description string
	Readme      string
	URL         string
	Closed      bool
	Owner       string
	Fields      []Field

	c *Client
}

// FieldByName returns the field with the given name (case-insensitive).
func (p *Project) FieldByName(name string) (Field, error) {
	for _, f := range p.Fields {
		if strings.EqualFold(f.Name, name) {
			return f, nil
		}
	}
	return Field{}, fmt.Errorf("no field named %q in project #%d", name, p.Number)
}

// StatusField returns the project's "Status" single-select field, which models
// the board's columns.
func (p *Project) StatusField() (Field, error) {
	return p.FieldByName("Status")
}

// projectGraphQL is the shared selection set for a project and its fields.
const projectGraphQL = `
  id number title shortDescription readme url closed
  fields(first:50){
    nodes{
      __typename
      ... on ProjectV2FieldCommon { id name dataType }
      ... on ProjectV2SingleSelectField { options { id name } }
      ... on ProjectV2IterationField {
        configuration { iterations { id title } completedIterations { id title } }
      }
    }
  }`

type rawProject struct {
	ID               string
	Number           int
	Title            string
	ShortDescription string
	Readme           string
	URL              string
	Closed           bool
	Fields           struct {
		Nodes []struct {
			Typename      string `json:"__typename"`
			ID            string
			Name          string
			DataType      string
			Options       []Option
			Configuration struct {
				Iterations          []Iteration
				CompletedIterations []Iteration
			}
		}
	}
}

func (rp rawProject) toProject(owner string, c *Client) *Project {
	p := &Project{
		ID:          rp.ID,
		Number:      rp.Number,
		Title:       rp.Title,
		Description: rp.ShortDescription,
		Readme:      rp.Readme,
		URL:         rp.URL,
		Closed:      rp.Closed,
		Owner:       owner,
		c:           c,
	}
	for _, n := range rp.Fields.Nodes {
		if n.ID == "" {
			continue // not a ProjectV2FieldCommon (shouldn't happen)
		}
		f := Field{ID: n.ID, Name: n.Name, DataType: n.DataType, Options: n.Options}
		f.Iterations = append(f.Iterations, n.Configuration.Iterations...)
		f.Iterations = append(f.Iterations, n.Configuration.CompletedIterations...)
		p.Fields = append(p.Fields, f)
	}
	return p
}

// Project fetches a single project by owner and number, resolving its fields.
func (c *Client) Project(owner string, number int) (*Project, error) {
	kind, err := c.resolveOwnerKind(owner)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`query($login:String!, $number:Int!){
      %s(login:$login){ projectV2(number:$number){ %s } }
    }`, kind, projectGraphQL)

	var resp struct {
		User         *struct{ ProjectV2 *rawProject } `json:"user"`
		Organization *struct{ ProjectV2 *rawProject } `json:"organization"`
	}
	vars := map[string]any{"login": owner, "number": number}
	if err := c.do(query, vars, &resp); err != nil {
		return nil, err
	}

	var rp *rawProject
	if resp.User != nil {
		rp = resp.User.ProjectV2
	} else if resp.Organization != nil {
		rp = resp.Organization.ProjectV2
	}
	if rp == nil || rp.ID == "" {
		return nil, fmt.Errorf("project #%d not found for owner %q", number, owner)
	}
	return rp.toProject(owner, c), nil
}

// ProjectSummary is a lightweight project listing entry.
type ProjectSummary struct {
	Number    int      `json:"number"`
	Title     string   `json:"title"`
	ID        string   `json:"id"`
	Closed    bool     `json:"closed"`
	ItemCount int      `json:"itemCount"`
	Repos     []string `json:"repos"`
}

// List returns the projects owned by the given login.
func (c *Client) List(owner string) ([]ProjectSummary, error) {
	kind, err := c.resolveOwnerKind(owner)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`query($login:String!, $cursor:String){
      %s(login:$login){
        projectsV2(first:100, after:$cursor){
          pageInfo{ hasNextPage endCursor }
          nodes{
            number title id closed
            items{ totalCount }
            repositories(first:20){ nodes{ nameWithOwner } }
          }
        }
      }
    }`, kind)

	type ownerNode struct {
		ProjectsV2 struct {
			PageInfo struct {
				HasNextPage bool
				EndCursor   string
			}
			Nodes []struct {
				Number       int
				Title        string
				ID           string
				Closed       bool
				Items        struct{ TotalCount int }
				Repositories struct {
					Nodes []struct{ NameWithOwner string }
				}
			}
		}
	}

	var out []ProjectSummary
	var cursor *string
	for {
		var resp struct {
			User         *ownerNode `json:"user"`
			Organization *ownerNode `json:"organization"`
		}
		vars := map[string]any{"login": owner, "cursor": cursor}
		if err := c.do(query, vars, &resp); err != nil {
			return nil, err
		}
		node := resp.User
		if node == nil {
			node = resp.Organization
		}
		if node == nil {
			break
		}
		for _, n := range node.ProjectsV2.Nodes {
			ps := ProjectSummary{Number: n.Number, Title: n.Title, ID: n.ID, Closed: n.Closed, ItemCount: n.Items.TotalCount}
			for _, r := range n.Repositories.Nodes {
				ps.Repos = append(ps.Repos, r.NameWithOwner)
			}
			out = append(out, ps)
		}
		if !node.ProjectsV2.PageInfo.HasNextPage {
			break
		}
		c := node.ProjectsV2.PageInfo.EndCursor
		cursor = &c
	}
	return out, nil
}
