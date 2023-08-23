package gerrit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/andygrunwald/go-gerrit"
	"kubevirt.io/project-infra/robots/pkg/flakefinder/api"
)

type Change struct {
	Number int
	SHA    string
	Ref    string
}

func (c *Change) Matches(status *api.StartedStatus) bool {
	fmt.Println(status)
	fmt.Println(c.Number)
	fmt.Println(c.SHA)
	fmt.Println(c.Ref)
	for _, v := range status.Repos {
		if strings.Contains(v, c.Ref) {
			return true
		}
	}
	return false
}

func (c *Change) ID() int {
	return c.Number
}

type Query struct {
	client *gerrit.Client
	repo   string
	branch string
}

func NewQuery(client *gerrit.Client, repo string, branch string) *Query {
	return &Query{
		client: client,
		repo:   repo,
		branch: branch,
	}
}

func (q *Query) Query(_ context.Context, startOfReport time.Time, endOfReport time.Time) ([]api.Change, error) {
	startBytes, _ := gerrit.Timestamp{startOfReport}.MarshalJSON()
	endBytes, _ := gerrit.Timestamp{endOfReport}.MarshalJSON()

	query := fmt.Sprintf("is:merged mergedafter:%v mergedbefore:%v branch:%v project:%v", string(startBytes), string(endBytes), q.branch, q.repo)
	fmt.Println(query)

	baseURL := q.client.BaseURL()
	gerritChanges := []*gerrit.ChangeInfo{}
	err := execQuery(baseURL.String(), "changes", query, &gerritChanges, "o=CURRENT_REVISION")
	if err != nil {
		return nil, err
	}

	changes := []api.Change{}
	for _, c := range gerritChanges {
		submitted := *c.Submitted
		ref := c.Revisions[c.CurrentRevision].Ref
		if submitted.Sub(c.Revisions[c.CurrentRevision].Created.Time) < 1*time.Second {
			ref = strings.TrimSuffix(ref, strconv.Itoa(c.Revisions[c.CurrentRevision].Number))
			ref = fmt.Sprintf("%s%d", ref, c.Revisions[c.CurrentRevision].Number-1)
			fmt.Println(ref)
		}

		changes = append(changes, &Change{
			Number: c.Number,
			SHA:    c.CurrentRevision,
			Ref:    ref,
		})
	}

	return changes, nil
}

func execQuery(baseURL string, resource string, query string, target interface{}, options string) error {
	u := baseURL + resource + "?q=" + url.QueryEscape(query)

	if options != "" {
		u = u + "&" + options
	}

	fmt.Println(u)
	cmd := exec.Command("gob-curl", u)
	resp, err := cmd.Output()
	if err != nil {
		return err
	}
	return json.Unmarshal(resp[4:], target)
}
