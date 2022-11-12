package main

import "time"

type Notification struct {
	Title    string
	Content  string
	PostTime time.Time
}

type Notifications []Notification
