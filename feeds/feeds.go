package feeds

type FeedConfig struct {
	URL             string
	Header          string
	Agent           string // // "bot", "chrome", "reader"
	EnhancedHeaders bool   // When true, use enhanced headers for the request
}

var Feeds = []FeedConfig{
	// {
	// 	URL:             "https://foro.elhacker.net/.xml.html;sa=news;board=34;limit=10;type=rss",
	// 	Header:          "elhacker",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://feeds.feedburner.com/TheHackersNews",
	// 	Header:          "The Hacker News",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	{
		URL:             "https://techmeme.com/feed.xml",
		Header:          "Techmeme",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	{
		URL:             "https://rss.slashdot.org/Slashdot/slashdotMain",
		Header:          "Slashdot",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	{
		URL:             "https://hnrss.org/frontpage",
		Header:          "Hacker News",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	{
		URL:             "https://tldr.tech/api/rss/tech",
		Header:          "TLDR",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	{
		URL:             "https://tldr.tech/api/rss/ai",
		Header:          "TLDR",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	{
		URL:             "https://tldr.tech/api/rss/founders",
		Header:          "TLDR",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	{
		URL:             "https://tldr.tech/api/rss/webdev",
		Header:          "TLDR",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	{
		URL:             "https://tldr.tech/api/rss/infosec",
		Header:          "TLDR",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	{
		URL:             "https://tldr.tech/api/rss/marketing",
		Header:          "TLDR",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	// {
	// 	URL:             "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=100727362",
	// 	Header:          "CNBC",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://search.cnbc.com/rs/search/combinedcms/view.xml?partnerId=wrss01&id=10000664",
	// 	Header:          "CNBC",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://www.ft.com/world?format=rss",
	// 	Header:          "FT",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://www.ft.com/markets?format=rss",
	// 	Header:          "FT",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://www.cityam.com/feed/",
	// 	Header:          "CityAM",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://www.investing.com/rss/news.rss",
	// 	Header:          "investing.com",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	{
		URL:             "https://www.investing.com/rss/investing_news.rss",
		Header:          "investing.com",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	// {
	// 	URL:             "https://www.propublica.org/feeds",
	// 	Header:          "ProPublica",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://rss.nytimes.com/services/xml/rss/nyt/World.xml",
	// 	Header:          "NYT",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://rss.nytimes.com/services/xml/rss/nyt/Politics.xml",
	// 	Header:          "NYT",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://www.washingtonpost.com/arcio/rss/category/world/",
	// 	Header:          "Washington Post",
	// 	Agent:           "chrome",
	// 	EnhancedHeaders: true,
	// },
	// {
	// 	URL:             "https://www.washingtonpost.com/arcio/rss/category/politics/",
	// 	Header:          "Washington Post",
	// 	Agent:           "chrome",
	// 	EnhancedHeaders: true,
	// },
	// {
	// 	URL:             "https://www.theguardian.com/world/rss",
	// 	Header:          "Guardian",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://www.theguardian.com/uk-news/rss",
	// 	Header:          "Guardian",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://www.theguardian.com/uk/business/rss",
	// 	Header:          "Guardian",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	{
		URL:             "https://antiwar.com/feeds",
		Header:          "Antiwar",
		Agent:           "bot",
		EnhancedHeaders: false,
	},
	// {
	// 	URL:             "https://www.reddit.com/r/worldnews/.rss",
	// 	Header:          "r/worldnews",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://www.reddit.com/r/geopolitics/.rss",
	// 	Header:          "r/geopolitics",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://www.reddit.com/r/anime_titties/.rss",
	// 	Header:          "r/anime_titties",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://hypebeast.com/feed",
	// 	Header:          "Hypebeast",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
	// {
	// 	URL:             "https://www.highsnobiety.com/feeds/rss",
	// 	Header:          "Highsnobiety",
	// 	Agent:           "bot",
	// 	EnhancedHeaders: false,
	// },
}
