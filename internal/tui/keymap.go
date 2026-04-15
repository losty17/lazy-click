package tui

type Keymap struct {
	Down       string
	Up         string
	Search     string
	Edit       string
	AddComment string
}

func DefaultKeymap() Keymap {
	return Keymap{
		Down:       "j",
		Up:         "k",
		Search:     "/",
		Edit:       "i",
		AddComment: "c",
	}
}
