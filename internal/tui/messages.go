package tui

type MouseActionMsg struct {
	Action MouseAction
	X      int
	Y      int
}

type StoreResultMsg struct {
	Error error
}

type PaletteToggleMsg struct {
	Open bool
}

type HandoffGeneratedMsg struct {
	Content string
	Error   error
}
