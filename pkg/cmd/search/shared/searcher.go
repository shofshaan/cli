package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/cli/cli/v2/internal/ghinstance"
	"github.com/cli/cli/v2/pkg/search"
)

const maxPerPage = 100

var linkRE = regexp.MustCompile(`<([^>]+)>;\s*rel="([^"]+)"`)
var pageRE = regexp.MustCompile(`(\?|&)page=(\d*)`)
var jsonTypeRE = regexp.MustCompile(`[/+]json($|;)`)

type searcher struct {
	client *http.Client
	host   string
}

func NewSearcher(client *http.Client, host string) search.Searcher {
	return &searcher{
		client: client,
		host:   host,
	}
}

func (s *searcher) Repositories(query search.Query) (search.RepositoriesResult, error) {
	result := search.RepositoriesResult{}
	toRetrieve := query.Limit
	var resp *http.Response
	var err error
	for toRetrieve > 0 {
		query.Limit = min(toRetrieve, maxPerPage)
		query.Page = nextPage(resp)
		if query.Page == 0 {
			break
		}
		page := search.RepositoriesResult{}
		resp, err = s.search(query, &page)
		if err != nil {
			return result, err
		}
		result.IncompleteResults = page.IncompleteResults
		result.Total = page.Total
		result.Items = append(result.Items, page.Items...)
		toRetrieve = toRetrieve - len(page.Items)
	}
	return result, nil
}

func nextPage(resp *http.Response) (page int) {
	page = 0
	if resp == nil {
		page = 1
	} else {
		for _, m := range linkRE.FindAllStringSubmatch(resp.Header.Get("Link"), -1) {
			if len(m) > 2 && m[2] == "next" {
				p := pageRE.FindStringSubmatch(m[1])
				if len(p) == 3 {
					i, err := strconv.Atoi(p[2])
					if err == nil {
						page = i
					}
				}
			}
		}
	}
	return
}

func (s *searcher) search(query search.Query, result interface{}) (*http.Response, error) {
	path := fmt.Sprintf("%ssearch/%s", ghinstance.RESTPrefix(s.host), query.Kind)
	qs := url.Values{}
	qs.Set("page", strconv.Itoa(query.Page))
	qs.Set("per_page", strconv.Itoa(query.Limit))
	qs.Set("q", s.QueryString(query))
	if query.Order.IsSet() {
		qs.Set(query.Order.Key(), query.Order.String())
	}
	if query.Sort.IsSet() {
		qs.Set(query.Sort.Key(), query.Sort.String())
	}
	url := fmt.Sprintf("%s?%s", path, qs.Encode())
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	if !success {
		return resp, handleHTTPError(resp)
	}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(result)
	if err != nil {
		return resp, err
	}
	return resp, nil
}

func (s *searcher) QueryString(query search.Query) string {
	q := strings.Builder{}
	quotedKeywords := quoteKeywords(query.Keywords)
	q.WriteString(strings.Join(quotedKeywords, " "))
	for k, v := range listSet(query.Qualifiers) {
		v = quoteQualifier(v)
		q.WriteString(fmt.Sprintf(" %s:%s", k, v))
	}
	return q.String()
}

func (s *searcher) URL(query search.Query) string {
	path := fmt.Sprintf("https://%s/search", s.host)
	queryString := url.Values{}
	queryString.Set("type", query.Kind)
	queryString.Set("q", s.QueryString(query))
	if query.Order.IsSet() {
		queryString.Set(query.Order.Key(), query.Order.String())
	}
	if query.Sort.IsSet() {
		queryString.Set(query.Sort.Key(), query.Sort.String())
	}
	url := fmt.Sprintf("%s?%s", path, queryString.Encode())
	return url
}

func quoteKeywords(ks []string) []string {
	for i, k := range ks {
		ks[i] = quoteKeyword(k)
	}
	return ks
}

func quoteKeyword(k string) string {
	if strings.ContainsAny(k, " \"\t\r\n") {
		if strings.Contains(k, ":") {
			z := strings.SplitN(k, ":", 2)
			return fmt.Sprintf("%s:%q", z[0], z[1])
		}
		return fmt.Sprintf("%q", k)
	}
	return k
}

func quoteQualifier(q string) string {
	if strings.ContainsAny(q, " \"\t\r\n") {
		return fmt.Sprintf("%q", q)
	}
	return q
}

func listSet(q search.Qualifiers) map[string]string {
	m := map[string]string{}
	for _, v := range q {
		if v.IsSet() {
			m[v.Key()] = v.String()
		}
	}
	return m
}

type httpError struct {
	Errors     []httpErrorItem
	Message    string
	RequestURL *url.URL
	StatusCode int
}

type httpErrorItem struct {
	Code     string
	Field    string
	Message  string
	Resource string
}

func (err httpError) Error() string {
	if err.StatusCode != 422 {
		return fmt.Sprintf("HTTP %d: %s (%s)", err.StatusCode, err.Message, err.RequestURL)
	}
	query := strings.TrimSpace(err.RequestURL.Query().Get("q"))
	return fmt.Sprintf("Invalid search query %q.\n%s", query, err.Errors[0].Message)
}

func handleHTTPError(resp *http.Response) error {
	httpError := httpError{
		RequestURL: resp.Request.URL,
		StatusCode: resp.StatusCode,
	}
	if !jsonTypeRE.MatchString(resp.Header.Get("Content-Type")) {
		httpError.Message = resp.Status
		return httpError
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, &httpError); err != nil {
		return err
	}
	return httpError
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
