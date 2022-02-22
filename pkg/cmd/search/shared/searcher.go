package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/cli/cli/v2/internal/ghinstance"
	"github.com/cli/cli/v2/pkg/search"
)

const (
	maxPerPage = 100
	orderKey   = "order"
	sortKey    = "sort"
)

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
	qs.Set("q", s.String(query))
	if query.Order != "" {
		qs.Set(orderKey, query.Order)
	}
	if query.Sort != "" {
		qs.Set(sortKey, query.Sort)
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

func (s *searcher) String(query search.Query) string {
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
	qs := url.Values{}
	qs.Set("type", query.Kind)
	qs.Set("q", s.String(query))
	if query.Order != "" {
		qs.Set(orderKey, query.Order)
	}
	if query.Sort != "" {
		qs.Set(sortKey, query.Sort)
	}
	url := fmt.Sprintf("%s?%s", path, qs.Encode())
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

// Copied from:
// https://github.com/asaskevich/govalidator/blob/f21760c49a8d602d863493de796926d2a5c1138d/utils.go#L107
func camelCaseToDash(str string) string {
	var output []rune
	var segment []rune
	for _, r := range str {
		// not treat number as separate segment
		if !unicode.IsLower(r) && string(r) != "-" && !unicode.IsNumber(r) {
			output = addSegment(output, segment)
			segment = nil
		}
		segment = append(segment, unicode.ToLower(r))
	}
	output = addSegment(output, segment)
	return string(output)
}

func addSegment(inrune, segment []rune) []rune {
	if len(segment) == 0 {
		return inrune
	}
	if len(inrune) != 0 {
		inrune = append(inrune, '-')
	}
	inrune = append(inrune, segment...)
	return inrune
}

func listSet(q search.Qualifiers) map[string]string {
	m := map[string]string{}
	v := reflect.ValueOf(q)
	t := reflect.TypeOf(q)
	for i := 0; i < v.NumField(); i++ {
		fieldName := t.Field(i).Name
		key := camelCaseToDash(fieldName)
		typ := v.FieldByName(fieldName).Kind()
		value := v.FieldByName(fieldName)
		switch typ {
		case reflect.Ptr:
			if value.IsNil() {
				continue
			}
			v := reflect.Indirect(value)
			m[key] = fmt.Sprintf("%v", v)
		case reflect.Slice:
			if value.IsNil() {
				continue
			}
			s := []string{}
			for i := 0; i < value.Len(); i++ {
				s = append(s, fmt.Sprintf("%v", value.Index(i)))
			}
			m[key] = strings.Join(s, ",")
		default:
			if value.IsZero() {
				continue
			}
			m[key] = fmt.Sprintf("%v", value)
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
