package tui

type Keymap struct {
	Down        string
	Up          string
	Search      string
	ListSearch  string
	Edit        string
	RefreshTask string
	AddComment  string
	Filter      string
	Favorite    string
	SortLists   string
	FavOnly     string
}

func DefaultKeymap() Keymap {
	return Keymap{
		Down:        "j",
		Up:          "k",
		Search:      "/",
		ListSearch:  "?",
		Edit:        "i",
		RefreshTask: "R",
		AddComment:  "c",
		Filter:      "f",
		Favorite:    "*",
		SortLists:   "o",
		FavOnly:     "v",
	}
}
