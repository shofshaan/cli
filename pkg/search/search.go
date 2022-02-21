package search

import "time"

type Qualifiers map[string]Qualifier

type Query struct {
	Keywords   []string
	Kind       string
	Limit      int
	Order      Parameter
	Page       int
	Qualifiers Qualifiers
	Sort       Parameter
}

type RepositoriesResult struct {
	IncompleteResults bool         `json:"incomplete_results"`
	Items             []Repository `json:"items"`
	Total             int          `json:"total_count"`
}

type Repository struct {
	Archived        bool      `json:"archived"`
	CreatedAt       time.Time `json:"created_at"`
	DefaultBranch   string    `json:"default_branch"`
	Description     string    `json:"description"`
	Disabled        bool      `json:"disabled"`
	Fork            bool      `json:"fork"`
	ForksCount      int       `json:"forks_count"`
	FullName        string    `json:"full_name"`
	HasDownloads    bool      `json:"has_downloads"`
	HasIssues       bool      `json:"has_issues"`
	HasPages        bool      `json:"has_pages"`
	HasProjects     bool      `json:"has_projects"`
	HasWiki         bool      `json:"has_wiki"`
	Homepage        string    `json:"homepage"`
	ID              int64     `json:"id"`
	Language        string    `json:"language"`
	License         License   `json:"license"`
	MasterBranch    string    `json:"master_branch"`
	Name            string    `json:"name"`
	OpenIssuesCount int       `json:"open_issues_count"`
	Owner           User      `json:"owner"`
	Private         bool      `json:"private"`
	PushedAt        time.Time `json:"pushed_at"`
	Size            int       `json:"size"`
	StargazersCount int       `json:"stargazers_count"`
	UpdatedAt       time.Time `json:"updated_at"`
	WatchersCount   int       `json:"watchers_count"`

	// URLs
	ArchiveURL       string `json:"archive_url"`
	AssigneesURL     string `json:"assignees_url"`
	BlobsURL         string `json:"blobs_url"`
	BranchesURL      string `json:"branches_url"`
	CloneURL         string `json:"clone_url"`
	CollaboratorsURL string `json:"collaborators_url"`
	CommentsURL      string `json:"comments_url"`
	CommitsURL       string `json:"commits_url"`
	CompareURL       string `json:"compare_url"`
	ContentsURL      string `json:"contents_url"`
	ContributorsURL  string `json:"contributors_url"`
	DeploymentsURL   string `json:"deployments_url"`
	DownloadsURL     string `json:"downloads_url"`
	EventsURL        string `json:"events_url"`
	ForksURL         string `json:"forks_url"`
	GitCommitsURL    string `json:"git_commits_url"`
	GitRefsURL       string `json:"git_refs_url"`
	GitTagsURL       string `json:"git_tags_url"`
	GitURL           string `json:"git_url"`
	HTMLURL          string `json:"html_url"`
	HooksURL         string `json:"hooks_url"`
	IssueCommentURL  string `json:"issue_comment_url"`
	IssueEventsURL   string `json:"issue_events_url"`
	IssuesURL        string `json:"issues_url"`
	KeysURL          string `json:"keys_url"`
	LabelsURL        string `json:"labels_url"`
	LanguagesURL     string `json:"languages_url"`
	MergesURL        string `json:"merges_url"`
	MilestonesURL    string `json:"milestones_url"`
	MirrorURL        string `json:"mirror_url"`
	NotificationsURL string `json:"notifications_url"`
	PullsURL         string `json:"pulls_url"`
	ReleasesURL      string `json:"releases_url"`
	SSHURL           string `json:"ssh_url"`
	SVNURL           string `json:"svn_url"`
	StargazersURL    string `json:"stargazers_url"`
	StatusesURL      string `json:"statuses_url"`
	SubscribersURL   string `json:"subscribers_url"`
	SubscriptionURL  string `json:"subscription_url"`
	TagsURL          string `json:"tags_url"`
	TeamsURL         string `json:"teams_url"`
	TreesURL         string `json:"trees_url"`
	URL              string `json:"url"`
}

type License struct {
	HTMLURL string `json:"html_url"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	URL     string `json:"url"`
}

type User struct {
	GravatarID string `json:"gravatar_id"`
	ID         int64  `json:"id"`
	Login      string `json:"login"`
	SiteAdmin  bool   `json:"site_admin"`
	Type       string `json:"type"`

	// URLs
	AvatarURL         string `json:"avatar_url"`
	EventsURL         string `json:"events_url"`
	FollowersURL      string `json:"followers_url"`
	FollowingURL      string `json:"following_url"`
	GistsURL          string `json:"gists_url"`
	HTMLURL           string `json:"html_url"`
	OrganizationsURL  string `json:"organizations_url"`
	ReceivedEventsURL string `json:"received_events_url"`
	ReposURL          string `json:"repos_url"`
	StarredURL        string `json:"starred_url"`
	SubscriptionsURL  string `json:"subscriptions_url"`
	URL               string `json:"url"`
}

type Searcher interface {
	Repositories(Query) (RepositoriesResult, error)
	URL(Query) string
}

// This is a superset of the pflag.Value interface.
// Set, String, and Type methods are needed to satisfy it.
type Qualifier interface {
	IsSet() bool
	Key() string
	Set(string) error
	String() string
	Type() string
}

type Parameter = Qualifier
