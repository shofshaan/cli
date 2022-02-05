package search

type Qualifiers map[string]Qualifier

type Query struct {
	Keywords   []string
	Kind       string
	Limit      int
	Order      Parameter
	Qualifiers Qualifiers
	Sort       Parameter
}

type Result struct {
	IncompleteResults bool                     `json:"incomplete_results"`
	Items             []map[string]interface{} `json:"items"`
	TotalCount        int                      `json:"total_count"`
}

type Searcher interface {
	Search(Query) (Result, error)
	URL(Query) string
}

type Qualifier interface {
	IsSet() bool
	Key() string
	Set(string) error
	String() string
	Type() string
}

type Parameter = Qualifier

func (q *Qualifiers) ListSet() map[string]string {
	m := map[string]string{}
	for _, v := range *q {
		if v.IsSet() {
			m[v.Key()] = v.String()
		}
	}
	return m
}
