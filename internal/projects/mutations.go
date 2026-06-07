package projects

import (
	"fmt"
	"strings"
)

// AddDraft adds a draft issue to the project and returns the new item id.
func (p *Project) AddDraft(title, body string) (string, error) {
	var resp struct {
		AddProjectV2DraftIssue struct {
			ProjectItem struct{ ID string }
		} `json:"addProjectV2DraftIssue"`
	}
	q := `mutation($p:ID!,$t:String!,$b:String!){
      addProjectV2DraftIssue(input:{projectId:$p,title:$t,body:$b}){ projectItem{ id } }
    }`
	vars := map[string]any{"p": p.ID, "t": title, "b": body}
	if err := p.c.do(q, vars, &resp); err != nil {
		return "", err
	}
	return resp.AddProjectV2DraftIssue.ProjectItem.ID, nil
}

// AddItem adds an existing issue/PR (by content node id) to the project and
// returns the new item id.
func (p *Project) AddItem(contentID string) (string, error) {
	var resp struct {
		AddProjectV2ItemByID struct {
			Item struct{ ID string }
		} `json:"addProjectV2ItemById"`
	}
	q := `mutation($p:ID!,$c:ID!){
      addProjectV2ItemById(input:{projectId:$p,contentId:$c}){ item{ id } }
    }`
	vars := map[string]any{"p": p.ID, "c": contentID}
	if err := p.c.do(q, vars, &resp); err != nil {
		return "", err
	}
	return resp.AddProjectV2ItemByID.Item.ID, nil
}

// RemoveItem deletes an item from the project (does not delete the issue/PR).
func (p *Project) RemoveItem(itemID string) error {
	var resp struct {
		DeleteProjectV2Item struct {
			DeletedItemID string
		} `json:"deleteProjectV2Item"`
	}
	q := `mutation($p:ID!,$i:ID!){
      deleteProjectV2Item(input:{projectId:$p,itemId:$i}){ deletedItemId }
    }`
	vars := map[string]any{"p": p.ID, "i": itemID}
	return p.c.do(q, vars, &resp)
}

// ConvertDraft converts a draft issue item into a real issue in the given repo.
func (p *Project) ConvertDraft(itemID, repoID string) error {
	var resp struct {
		ConvertProjectV2DraftIssueItemToIssue struct {
			Item struct{ ID string }
		} `json:"convertProjectV2DraftIssueItemToIssue"`
	}
	q := `mutation($i:ID!,$r:ID!){
      convertProjectV2DraftIssueItemToIssue(input:{itemId:$i,repositoryId:$r}){ item{ id } }
    }`
	vars := map[string]any{"i": itemID, "r": repoID}
	return p.c.do(q, vars, &resp)
}

// CreateIssue creates a real issue in repo and returns its content node id and
// number. Used by the `create` command before adding the issue to the board.
func (c *Client) CreateIssue(repoID, title, body string, labelIDs []string) (Content, error) {
	var resp struct {
		CreateIssue struct {
			Issue Content
		} `json:"createIssue"`
	}
	q := `mutation($r:ID!,$t:String!,$b:String!,$l:[ID!]){
      createIssue(input:{repositoryId:$r,title:$t,body:$b,labelIds:$l}){
        issue{ id number title url }
      }
    }`
	vars := map[string]any{"r": repoID, "t": title, "b": body, "l": labelIDs}
	if err := c.do(q, vars, &resp); err != nil {
		return Content{}, err
	}
	return resp.CreateIssue.Issue, nil
}

// fieldValueInput builds the ProjectV2FieldValue input fragment for a field of
// the given data type, choosing the right key (text/number/date/optionId/etc).
func fieldValueInput(f Field, value string) (string, map[string]any, error) {
	switch f.DataType {
	case "SINGLE_SELECT":
		opt, err := f.OptionByName(value)
		if err != nil {
			return "", nil, err
		}
		return "{singleSelectOptionId:$v}", map[string]any{"v": opt.ID}, nil
	case "TEXT", "TITLE":
		return "{text:$v}", map[string]any{"v": value}, nil
	case "NUMBER":
		var n float64
		if _, err := fmt.Sscanf(value, "%g", &n); err != nil {
			return "", nil, fmt.Errorf("field %q expects a number, got %q", f.Name, value)
		}
		return "{number:$v}", map[string]any{"v": n}, nil
	case "DATE":
		return "{date:$v}", map[string]any{"v": value}, nil
	case "ITERATION":
		for _, it := range f.Iterations {
			if strings.EqualFold(it.Title, value) {
				return "{iterationId:$v}", map[string]any{"v": it.ID}, nil
			}
		}
		return "", nil, fmt.Errorf("no iteration %q in field %q", value, f.Name)
	default:
		return "", nil, fmt.Errorf("setting field type %q is not supported", f.DataType)
	}
}

// SetFieldValue sets a field value on an item, interpreting value according to
// the field's data type (option name for single-select, etc).
func (p *Project) SetFieldValue(itemID string, f Field, value string) error {
	valFrag, valVars, err := fieldValueInput(f, value)
	if err != nil {
		return err
	}
	q := fmt.Sprintf(`mutation($p:ID!,$i:ID!,$f:ID!,$v:%s){
      updateProjectV2ItemFieldValue(input:{projectId:$p,itemId:$i,fieldId:$f,value:%s}){
        projectV2Item{ id }
      }
    }`, valVarType(f.DataType), valFrag)
	vars := map[string]any{"p": p.ID, "i": itemID, "f": f.ID}
	for k, v := range valVars {
		vars[k] = v
	}
	var resp struct {
		UpdateProjectV2ItemFieldValue struct {
			ProjectV2Item struct{ ID string }
		} `json:"updateProjectV2ItemFieldValue"`
	}
	return p.c.do(q, vars, &resp)
}

// valVarType returns the GraphQL type for the $v variable used by SetFieldValue.
func valVarType(dataType string) string {
	switch dataType {
	case "NUMBER":
		return "Float!"
	case "DATE":
		return "Date!"
	default:
		return "String!"
	}
}

// UpdateIssueBody edits the body of a real issue/PR (by content node id).
func (c *Client) UpdateIssueBody(contentID, body string) error {
	q := `mutation($id:ID!,$b:String!){
      updateIssue(input:{id:$id,body:$b}){ issue{ id } }
    }`
	vars := map[string]any{"id": contentID, "b": body}
	var resp any
	return c.do(q, vars, &resp)
}

// UpdateDraftBody edits the body of a draft issue (by draft node id).
func (c *Client) UpdateDraftBody(draftID, body string) error {
	q := `mutation($id:ID!,$b:String!){
      updateProjectV2DraftIssue(input:{draftIssueId:$id,body:$b}){ draftIssue{ id } }
    }`
	vars := map[string]any{"id": draftID, "b": body}
	var resp any
	return c.do(q, vars, &resp)
}

// CreateField creates a new field. For single-select fields, pass option names.
func (p *Project) CreateField(name, dataType string, options []string) (Field, error) {
	dt := strings.ToUpper(dataType)
	q := `mutation($p:ID!,$n:String!,$dt:ProjectV2CustomFieldType!,$opts:[ProjectV2SingleSelectFieldOptionInput!]){
      createProjectV2Field(input:{projectId:$p,name:$n,dataType:$dt,singleSelectOptions:$opts}){
        projectV2Field{
          __typename
          ... on ProjectV2FieldCommon { id name dataType }
          ... on ProjectV2SingleSelectField { options{ id name } }
        }
      }
    }`
	var opts []map[string]any
	if dt == "SINGLE_SELECT" {
		for _, o := range options {
			// color and description are required by the API.
			opts = append(opts, map[string]any{"name": o, "color": "GRAY", "description": ""})
		}
	}
	vars := map[string]any{"p": p.ID, "n": name, "dt": dt, "opts": opts}
	var resp struct {
		CreateProjectV2Field struct {
			ProjectV2Field struct {
				ID       string
				Name     string
				DataType string
				Options  []Option
			}
		} `json:"createProjectV2Field"`
	}
	if err := p.c.do(q, vars, &resp); err != nil {
		return Field{}, err
	}
	f := resp.CreateProjectV2Field.ProjectV2Field
	return Field{ID: f.ID, Name: f.Name, DataType: f.DataType, Options: f.Options}, nil
}

// DeleteField deletes a field by its node id.
func (p *Project) DeleteField(fieldID string) error {
	q := `mutation($f:ID!){ deleteProjectV2Field(input:{fieldId:$f}){ projectV2Field{ __typename } } }`
	vars := map[string]any{"f": fieldID}
	var resp any
	return p.c.do(q, vars, &resp)
}

// LinkRepo links the project to a repository (so it appears on that repo's
// Projects tab).
func (p *Project) LinkRepo(repoID string) error {
	q := `mutation($p:ID!,$r:ID!){ linkProjectV2ToRepository(input:{projectId:$p,repositoryId:$r}){ repository{ nameWithOwner } } }`
	vars := map[string]any{"p": p.ID, "r": repoID}
	var resp any
	return p.c.do(q, vars, &resp)
}

// UnlinkRepo removes the project's link to a repository.
func (p *Project) UnlinkRepo(repoID string) error {
	q := `mutation($p:ID!,$r:ID!){ unlinkProjectV2FromRepository(input:{projectId:$p,repositoryId:$r}){ repository{ nameWithOwner } } }`
	vars := map[string]any{"p": p.ID, "r": repoID}
	var resp any
	return p.c.do(q, vars, &resp)
}
