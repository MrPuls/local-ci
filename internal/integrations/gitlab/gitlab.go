package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type GitlabOptions struct {
	Url       string
	Token     string
	ProjectId int
}

type GitLabUtil struct {
	Url             string
	Token           string
	ProjectId       int
	apiPath         string
	apiProjectsPath string
	variablesSlug   string
	paginationQuery string
}

type Variable struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NewGitLabUtil(options *GitlabOptions) *GitLabUtil {
	return &GitLabUtil{
		Url:             options.Url,
		Token:           options.Token,
		ProjectId:       options.ProjectId,
		apiPath:         "/api/v4",
		apiProjectsPath: "/projects/",
		variablesSlug:   "/variables",
		paginationQuery: "?per_page=1000",
	}
}

func (g *GitLabUtil) GetRemoteAddress() string {
	return g.Url
}

func (g *GitLabUtil) GetRemoteVariables() map[string]string {
	url := g.assembleVariablesURL(g.ProjectId)
	res := make(map[string]string)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}

	if g.Token[0] == '$' {
		g.Token = os.Getenv(strings.TrimPrefix(g.Token, "$"))
	}

	req.Header.Add("PRIVATE-TOKEN", g.Token)
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	var r []Variable
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil
	}
	for _, v := range r {
		res[v.Key] = v.Value
	}

	defer resp.Body.Close()
	return res
}

func (g *GitLabUtil) constructURL(url string) string {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	return url
}

func (g *GitLabUtil) assembleVariablesURL(projectId int) string {
	return g.constructURL(g.Url) + g.apiPath + g.apiProjectsPath + fmt.Sprintf("%d", projectId) + g.variablesSlug + g.paginationQuery
}
