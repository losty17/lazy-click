package tui

type Keymap struct {
	Down         string
	Up           string
	Search       string
	TaskTitle    string
	CopyTaskLink string
	Edit         string
	RefreshTask  string
	AddComment   string
	Filter       string
	Favorite     string
	SortLists    string
	FavOnly      string
	SortTasks    string
	SortDirection string
	GroupTasks   string
	Subtasks     string
	CollapseAll  string
	MeMode             string
	DownloadAttachments string
}

func DefaultKeymap() Keymap {
	return Keymap{
		Down:                "down",
		Up:                  "up",
		Search:              "/",
		TaskTitle:           "t",
		CopyTaskLink:        "[",
		Edit:                "i",
		RefreshTask:         "R",
		AddComment:          "c",
		Filter:              "f",
		Favorite:            "*",
		SortLists:           "o",
		FavOnly:             "v",
		SortTasks:           "O",
		SortDirection:       "ctrl+o",
		GroupTasks:          "g",
		Subtasks:            "G",
		CollapseAll:         "X",
		MeMode:              "m",
		DownloadAttachments: "a",
	}
}
