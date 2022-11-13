package main

import (
	"encoding/json"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	site           string
	username       string
	password       string
	updateInterval = 15 * time.Second
	version        = "0.1.0"
	notes          []*fyne.Notification
	notelock       sync.Mutex
	checksums      []string
	endpoints      []string
)

func main() {
	a := app.NewWithID("com.notify.app")
	w := a.NewWindow("Notify")

	// Configuration Block
	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Username")
	usernameEntry.OnChanged = func(s string) {
		username = s
	}
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Password")
	passwordEntry.OnChanged = func(s string) {
		password = s
	}
	watchSiteEntry := widget.NewEntry()
	watchSiteEntry.SetPlaceHolder("Site")
	watchSiteEntry.OnChanged = func(s string) {
		err := watchSiteEntry.Validate()
		if err != nil {
			return
		}

		site = s
	}
	watchSiteEntry.Validator = func(s string) error {
		_, err := url.Parse(s)
		return err
	}
	updateIntervalEntry := widget.NewEntry()
	updateIntervalEntry.SetPlaceHolder("Update Interval - 1s, 1m, 1h")
	updateIntervalEntry.Validator = func(s string) error {
		_, err := time.ParseDuration(s)
		return err
	}
	updateIntervalEntry.OnChanged = func(s string) {
		err := updateIntervalEntry.Validate()
		if err != nil {
			updateInterval = 15 * time.Second
		} else {
			updateInterval, _ = time.ParseDuration(s)
		}
	}

	endpointEntry := widget.NewEntry()
	endpointEntry.SetPlaceHolder("Endpoint")
	endpointEntry.OnChanged = func(s string) {
		endpoints = strings.Split(strings.Replace(s, " ", "", -1), ",")

	}

	config := container.NewGridWithRows(5, watchSiteEntry, endpointEntry, usernameEntry, passwordEntry, updateIntervalEntry)
	configVscroll := container.NewVScroll(config)
	// Notification Block
	notificationList := widget.NewList(
		func() int {
			return len(notes)
		},
		func() fyne.CanvasObject {
			md := widget.NewRichTextFromMarkdown("string\n\nstring\n")
			md.Resize(fyne.Size{Height: md.Size().Height * 2, Width: md.Size().Width})
			return md
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.RichText).ParseMarkdown(notes[i].Title + "\n\n" + notes[i].Content)
		})

	notificationList.OnSelected = func(id widget.ListItemID) {
		d := dialog.NewConfirm("Confirm", "Mark as read?", func(b bool) {
			if b {
				notelock.Lock()
				notes = append(notes[:id], notes[id+1:]...)
				notelock.Unlock()
				notificationList.Refresh()
			}
		}, w)
		d.Show()
		notificationList.Unselect(id)
	}

	tabMenu := container.NewAppTabs(
		container.NewTabItem("Notifications", notificationList),
		container.NewTabItem("Config", configVscroll),
	)

	w.SetContent(tabMenu)

	go func() {
		for {
			notelock.Lock()
			newNotes := GetUpdates()

			for _, n := range newNotes {
				if len(notes) == 0 {
					notes = append(notes, n)
					notificationList.Refresh()

				} else {
					for _, note := range notes {
						if note.Content == n.Content && note.Title == n.Title {
							println("Duplicate")
							continue
						} else {
							a.SendNotification(n)
							println("New Notification")
							notes = append(notes, n)
						}
					}
				}
			}
			notelock.Unlock()
			time.Sleep(updateInterval)
			notificationList.Refresh()
		}
	}()

	w.ShowAndRun()
}

func GetUpdates() []*fyne.Notification {
	if site == "" || username == "" || password == "" || len(endpoints) == 0 {
		println("No site, username, or password")
		return []*fyne.Notification{fyne.NewNotification("Notify", "Please configure Notify")}
	}
	notifications := []*fyne.Notification{}
	for _, endpoint := range endpoints {
		data, err := BasicAuthGet(site, endpoint, username, password)
		if err != nil {
			return []*fyne.Notification{fyne.NewNotification("Error", err.Error())}
		} else {
			newNotifications := Notifications{}
			err = json.Unmarshal(data, &newNotifications)
			if err != nil {
				return []*fyne.Notification{fyne.NewNotification("Error", err.Error())}
			} else {

				for _, n := range newNotifications {
					newPost := true
					for _, c := range checksums {
						if c == n.Checksum {
							newPost = false
						}
					}
					if newPost {
						notifications = append(notifications, fyne.NewNotification(n.Title, n.Content))
						checksums = append(checksums, n.Checksum)
					}
				}

			}
		}
	}
	return notifications
}

func BasicAuthGet(url string, endpoint string, username string, password string) ([]byte, error) {
	req, err := http.NewRequest("GET", url+"/"+endpoint, nil)
	req.SetBasicAuth(username, password)
	req.Header.Set("User-Agent", "GO-Notify/"+version)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return body, nil
}
