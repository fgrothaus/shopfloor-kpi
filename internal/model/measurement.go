package model

import "time"

type Measurement struct {
	Machine   string
	Signal    string
	Value     any
	Good      bool
	Timestamp time.Time
}
