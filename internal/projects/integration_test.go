//go:build integration

// Live integration tests that exercise the real GitHub Projects API.
//
// They run only with `go test -tags integration` and require gh to be
// authenticated with the `project` scope. They operate on a disposable test
// project (default: #5) and clean up everything they create.
//
//	GHP_TEST_OWNER       project owner login        (default: authenticated user)
//	GHP_TEST_PROJECT     disposable project number  (default: 5)
//	GHP_TEST_REPO        repo for link/add tests    (default: NSExceptional/home; not modified)
//	GHP_TEST_ISSUE_REPO  throwaway repo for tests that CREATE issues (create/convert);
//	                     those tests skip unless this is set.
package projects

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testProject(t *testing.T) (*Client, *Project) {
	t.Helper()
	c, err := New()
	if err != nil {
		t.Skipf("no gh client (is gh authenticated?): %v", err)
	}
	owner := os.Getenv("GHP_TEST_OWNER")
	if owner == "" {
		if owner, err = c.Viewer(); err != nil {
			t.Skipf("cannot resolve viewer: %v", err)
		}
	}
	num := 5
	if s := os.Getenv("GHP_TEST_PROJECT"); s != "" {
		num, _ = strconv.Atoi(s)
	}
	p, err := c.Project(owner, num)
	if err != nil {
		t.Fatalf("open project #%d for %q: %v", num, owner, err)
	}
	return c, p
}

func testRepo() string {
	if r := os.Getenv("GHP_TEST_REPO"); r != "" {
		return r
	}
	return "NSExceptional/home"
}

// fetchItem reads a single item by its node id. Querying by id reflects writes
// far sooner than the project's items connection, which is eventually
// consistent (see the README note on read-after-write lag).
func fetchItem(c *Client, id string) (Item, error) {
	var resp struct {
		Node rawItem `json:"node"`
	}
	q := `query($id:ID!){ node(id:$id){ ... on ProjectV2Item { ` + itemGraphQL + ` } } }`
	if err := c.do(q, map[string]any{"id": id}, &resp); err != nil {
		return Item{}, err
	}
	return resp.Node.toItem(), nil
}

// retry polls fn until it reports done, the deadline passes, or it errors.
func retry(t *testing.T, what string, timeout time.Duration, fn func() (done bool, err error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		done, err := fn()
		if err != nil {
			t.Fatalf("%s: %v", what, err)
		}
		if done {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: timed out after %s", what, timeout)
		}
		time.Sleep(3 * time.Second)
	}
}

func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestIntegration_LinkUnlink(t *testing.T) {
	c, p := testProject(t)
	repo, err := c.Repo(testRepo())
	if err != nil {
		t.Fatal(err)
	}

	if err := p.LinkRepo(repo.ID); err != nil {
		t.Fatalf("link: %v", err)
	}
	t.Cleanup(func() { _ = p.UnlinkRepo(repo.ID) })

	retry(t, "linked repo appears", 60*time.Second, func() (bool, error) {
		repos, err := p.LinkedRepos()
		return sliceContains(repos, repo.NameWithOwner), err
	})

	if err := p.UnlinkRepo(repo.ID); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	retry(t, "linked repo removed", 60*time.Second, func() (bool, error) {
		repos, err := p.LinkedRepos()
		return !sliceContains(repos, repo.NameWithOwner), err
	})
}

func TestIntegration_FieldsAndSet(t *testing.T) {
	c, p := testProject(t)

	itemID, err := p.AddDraft("itest draft (set fields)", "body")
	if err != nil {
		t.Fatalf("add draft: %v", err)
	}
	t.Cleanup(func() { _ = p.RemoveItem(itemID) })

	cases := []struct {
		name, dtype, value string
		opts               []string
	}{
		{"itest-text", "TEXT", "hello world", nil},
		{"itest-number", "NUMBER", "42", nil},
		{"itest-date", "DATE", "2026-01-15", nil},
		{"itest-select", "SINGLE_SELECT", "Beta", []string{"Alpha", "Beta"}},
	}
	want := map[string]string{}
	for _, c := range cases {
		f, err := p.CreateField(c.name, c.dtype, c.opts)
		if err != nil {
			t.Fatalf("create field %s: %v", c.name, err)
		}
		fid := f.ID
		t.Cleanup(func() { _ = p.DeleteField(fid) })
		if err := p.SetFieldValue(itemID, f, c.value); err != nil {
			t.Fatalf("set %s: %v", c.name, err)
		}
		want[c.name] = c.value
	}

	retry(t, "all field values applied", 90*time.Second, func() (bool, error) {
		it, err := fetchItem(c, itemID)
		if err != nil {
			return false, err
		}
		for k, v := range want {
			if it.Fields[k] != v {
				return false, nil
			}
		}
		return true, nil
	})
}

func TestIntegration_AddExistingItem(t *testing.T) {
	c, p := testProject(t)

	content, err := c.Content(testRepo(), "1") // reference an existing issue; does not modify it
	if err != nil {
		t.Fatalf("resolve %s#1: %v", testRepo(), err)
	}
	itemID, err := p.AddItem(content.ID)
	if err != nil {
		t.Fatalf("add item: %v", err)
	}
	t.Cleanup(func() { _ = p.RemoveItem(itemID) })

	retry(t, "added item resolves to the issue", 60*time.Second, func() (bool, error) {
		it, err := fetchItem(c, itemID)
		return it.Number == content.Number && it.Type == "ISSUE", err
	})
}

func TestIntegration_DraftBodyToggle(t *testing.T) {
	c, p := testProject(t)

	itemID, err := p.AddDraft("itest draft (body)", "- [ ] task one\n- [ ] task two")
	if err != nil {
		t.Fatalf("add draft: %v", err)
	}
	t.Cleanup(func() { _ = p.RemoveItem(itemID) })

	var draftID string
	retry(t, "draft readable", 60*time.Second, func() (bool, error) {
		it, err := fetchItem(c, itemID)
		draftID = it.ContentID
		return it.ContentID != "" && it.Body != "", err
	})

	body, _, err := ToggleChecklist("- [ ] task one\n- [ ] task two", "task one", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateDraftBody(draftID, body); err != nil {
		t.Fatalf("update draft body: %v", err)
	}
	retry(t, "checkbox toggled", 60*time.Second, func() (bool, error) {
		it, err := fetchItem(c, itemID)
		return strings.Contains(it.Body, "- [x] task one"), err
	})
}

func TestIntegration_ResolveDraftByTitle(t *testing.T) {
	c, p := testProject(t)

	const title = "itest resolve by title"
	itemID, err := p.AddDraft(title, "body")
	if err != nil {
		t.Fatalf("add draft: %v", err)
	}
	t.Cleanup(func() { _ = p.RemoveItem(itemID) })

	retry(t, "draft resolvable by title substring", 60*time.Second, func() (bool, error) {
		it, err := p.ResolveItem("resolve by title")
		if err != nil {
			return false, nil // not indexed yet
		}
		return it.ID == itemID && it.IsDraft(), nil
	})

	// node-by-id read is immediate, so this also confirms resolution by id.
	if it, err := fetchItem(c, itemID); err != nil || it.ID != itemID {
		t.Fatalf("fetch by id: it=%+v err=%v", it, err)
	}
}

// TestIntegration_CreateAndConvert creates real issues, so it runs only when
// GHP_TEST_ISSUE_REPO points at a throwaway repo. It deletes what it creates.
func TestIntegration_CreateAndConvert(t *testing.T) {
	repoName := os.Getenv("GHP_TEST_ISSUE_REPO")
	if repoName == "" {
		t.Skip("set GHP_TEST_ISSUE_REPO to a throwaway repo to run issue-creating tests")
	}
	c, p := testProject(t)
	repo, err := c.Repo(repoName)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("create", func(t *testing.T) {
		issue, err := c.CreateIssue(repo.ID, "itest created issue", "body", nil)
		if err != nil {
			t.Fatalf("create issue: %v", err)
		}
		t.Cleanup(func() { ghIssueDelete(repoName, issue.Number) })
		itemID, err := p.AddItem(issue.ID)
		if err != nil {
			t.Fatalf("add created issue: %v", err)
		}
		t.Cleanup(func() { _ = p.RemoveItem(itemID) })
		retry(t, "created issue is on board", 60*time.Second, func() (bool, error) {
			it, err := fetchItem(c, itemID)
			return it.Number == issue.Number, err
		})
	})

	t.Run("convert", func(t *testing.T) {
		itemID, err := p.AddDraft("itest convert me", "converted body")
		if err != nil {
			t.Fatalf("add draft: %v", err)
		}
		t.Cleanup(func() { _ = p.RemoveItem(itemID) })
		if err := p.ConvertDraft(itemID, repo.ID); err != nil {
			t.Fatalf("convert: %v", err)
		}
		var number int
		retry(t, "draft converted to issue", 90*time.Second, func() (bool, error) {
			it, err := fetchItem(c, itemID)
			if err != nil {
				return false, err
			}
			if it.Type == "ISSUE" && it.Number > 0 {
				number = it.Number
				return true, nil
			}
			return false, nil
		})
		t.Cleanup(func() { ghIssueDelete(repoName, number) })
	})
}

func ghIssueDelete(repo string, number int) {
	if number <= 0 {
		return
	}
	_ = exec.Command("gh", "issue", "delete", strconv.Itoa(number), "-R", repo, "--yes").Run()
}
