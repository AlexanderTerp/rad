package core

import (
	tblwriter "github.com/amterp/go-tbl"
)

const (
	BLACK   = "black"
	RED     = "red"
	GREEN   = "green"
	YELLOW  = "yellow"
	BLUE    = "blue"
	MAGENTA = "magenta"
	CYAN    = "cyan"
	WHITE   = "white"
)

var COLORS = []string{BLACK, RED, GREEN, YELLOW, BLUE, MAGENTA, CYAN, WHITE}

var (
	Black   = tblwriter.Black
	Red     = tblwriter.Red
	Green   = tblwriter.Green
	Yellow  = tblwriter.Yellow
	Blue    = tblwriter.Blue
	Magenta = tblwriter.Magenta
	Cyan    = tblwriter.Cyan
	White   = tblwriter.White
)

func ColorFromString(s string) (tblwriter.Color, bool) {
	switch s {
	case BLACK:
		return Black, true
	case RED:
		return Red, true
	case GREEN:
		return Green, true
	case YELLOW:
		return Yellow, true
	case BLUE:
		return Blue, true
	case MAGENTA:
		return Magenta, true
	case CYAN:
		return Cyan, true
	case WHITE:
		return White, true
	default:
		return tblwriter.Plain, false
	}
}
