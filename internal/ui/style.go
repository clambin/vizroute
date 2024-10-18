package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type theme struct {
	HeaderFgColor tcell.Color
	HeaderBgColor tcell.Color
	CellFgColor   tcell.Color
	CellBgColor   tcell.Color
}

var style = theme{
	HeaderFgColor: tcell.ColorWhite,
	HeaderBgColor: tcell.ColorBlack,
	CellFgColor:   tcell.ColorSkyblue,
	CellBgColor:   tcell.ColorBlack,
}

func init() {
	tview.Styles.BorderColor = tcell.ColorSkyblue
}
