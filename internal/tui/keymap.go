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
	Debug               string
	CreateTask          string
	DeleteTask          string
	CreateList          string
	DeleteList          string
	DeleteComment       string
	ViewMode            string
	OpenTaskInBrowser   string
	StartTimeTracking   string
	StopTimeTracking    string
	ManageTimeEntries   string
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
		Debug:               "ctrl+d",
		CreateTask:          "C",
		DeleteTask:          "D",
		CreateList:          "E",
		DeleteList:          "L",
		DeleteComment:       "K",
		ViewMode:            "V",
		OpenTaskInBrowser:   "shift+enter",
		StartTimeTracking:   "w",
		StopTimeTracking:    "W",
		ManageTimeEntries:   "T",
	}
}
