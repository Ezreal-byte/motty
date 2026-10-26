package i18n

// AboutMessages are the texts of the About page of the settings: the
// application's version, where to talk about it and where it is developed.
type AboutMessages struct {
	Title string // the settings category, and the page's title

	Tagline string

	// The facts of the build. BuildDevelopment stands in for the commit when
	// this copy was built without one.
	InfoTitle        string
	Platform         string
	Build            string
	BuildDevelopment string
	Runtime          string

	// Copying those facts for a bug report.
	CopyRow         string
	CopyDescription string
	CopyAction      string
	CopiedAction    string // the button's label once the facts are on the clipboard
	Copied          string

	// Where to ask, and what each place is for.
	CommunityTitle     string
	QQ                 string
	QQDescription      string
	WeChat             string
	WeChatDescription  string
	Discord            string
	DiscordDescription string
	Issues             string
	IssuesDescription  string
	ProjectTitle       string
	Repo               string
	RepoDescription    string
	Docs               string
	DocsDescription    string
	Join               string
	Open               string
}
