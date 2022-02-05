package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/cli/cli/v2/pkg/search"
)

type searcher struct {
	client *http.Client
	host   string
}

func NewSearcher(client *http.Client, host string) *searcher {
	return &searcher{
		client: client,
		host:   host,
	}
}

func (s *searcher) Search(query search.Query) (search.Result, error) {
	result := search.Result{}
	path := fmt.Sprintf("https://api.%s/search/%s", s.host, query.Kind)
	queryString := url.Values{}
	q := strings.Builder{}
	quotedKeywords := quoteKeywords(query.Keywords)
	q.WriteString(strings.Join(quotedKeywords, " "))
	for k, v := range query.Qualifiers.ListSet() {
		v = quoteQualifier(v)
		q.WriteString(fmt.Sprintf(" %s:%s", k, v))
	}
	queryString.Set("q", q.String())
	if query.Order.IsSet() {
		queryString.Set(query.Order.Key(), query.Order.String())
	}
	if query.Sort.IsSet() {
		queryString.Set(query.Sort.Key(), query.Sort.String())
	}
	pages := math.Ceil((float64(query.Limit) / float64(100)))
	for i := 1; i <= int(pages); i++ {
		queryString.Set("page", strconv.Itoa(i))
		remaining := query.Limit - (i * 100)
		if remaining > 100 {
			queryString.Set("per_page", "100")
		} else if remaining <= 0 {
			queryString.Set("per_page", strconv.Itoa(query.Limit))
		} else {
			queryString.Set("per_page", strconv.Itoa(remaining))
		}
		url := fmt.Sprintf("%s?%s", path, queryString.Encode())
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return result, err
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		req.Header.Set("Accept", "application/vnd.github.v3+json")
		resp, err := s.client.Do(req)
		if err != nil {
			return result, err
		}
		defer resp.Body.Close()
		success := resp.StatusCode >= 200 && resp.StatusCode < 300
		if !success {
			return result, handleHTTPError(resp)
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return result, err
		}
		pageResult := search.Result{}
		err = json.Unmarshal(b, &pageResult)
		if err != nil {
			return result, err
		}
		result.IncompleteResults = pageResult.IncompleteResults
		result.TotalCount = pageResult.TotalCount
		result.Items = append(result.Items, pageResult.Items...)
	}
	return result, nil
}

func (s *searcher) URL(query search.Query) string {
	path := fmt.Sprintf("https://%s/search", s.host)
	queryString := url.Values{}
	queryString.Set("type", query.Kind)
	if query.Order.IsSet() {
		queryString.Set(query.Order.Key(), query.Order.String())
	}
	if query.Sort.IsSet() {
		queryString.Set(query.Sort.Key(), query.Sort.String())
	}
	q := strings.Builder{}
	quotedKeywords := quoteKeywords(query.Keywords)
	q.WriteString(strings.Join(quotedKeywords, " "))
	for k, v := range query.Qualifiers.ListSet() {
		v = quoteQualifier(v)
		q.WriteString(fmt.Sprintf(" %s:%s", k, v))
	}
	queryString.Add("q", q.String())
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

var jsonTypeRE = regexp.MustCompile(`[/+]json($|;)`)

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
