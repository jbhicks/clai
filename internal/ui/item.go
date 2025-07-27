package ui

type Item string

func (i Item) FilterValue() string { return "" }

func (i Item) Title() string { return string(i) }

func (i Item) Description() string { return "" }
