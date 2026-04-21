package tui

type Keymap struct {
	Down        string
	Up          string
	Search      string
	ListSearch  string
	TaskTitle   string
	Edit        string
	RefreshTask string
	AddComment  string
	Filter      string
	Favorite    string
	SortLists   string
	FavOnly     string
	SortTasks   string
	GroupTasks  string
	Subtasks    string
	CollapseAll string
}

func DefaultKeymap() Keymap {
	return Keymap{
		Down:        "j",
		Up:          "k",
		Search:      "/",
		ListSearch:  "?",
		TaskTitle:   "t",
		Edit:        "i",
		RefreshTask: "R",
		AddComment:  "c",
		Filter:      "f",
		Favorite:    "*",
		SortLists:   "o",
		FavOnly:     "v",
		SortTasks:   "O",
		GroupTasks:  "g",
		Subtasks:    "G",
		CollapseAll: "X",
	}
}
