package main

import (
	"encoding/json"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"io"
	"net/http"
	"net/url"
	"time"
)

var (
	site           string
	username       string
	password       string
	updateInterval = 15 * time.Second
	version        = "0.1.0"
	notes          []*fyne.Notification
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

	config := container.NewGridWithRows(4, watchSiteEntry, usernameEntry, passwordEntry, updateIntervalEntry)
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

	tabMenu := container.NewAppTabs(
		container.NewTabItem("Notifications", notificationList),
		container.NewTabItem("Config", configVscroll),
	)

	w.SetContent(tabMenu)

	go func() {
		for {
			for _, n := range GetUpdates() {
				a.SendNotification(n)
			}
			notes = GetUpdates()
			time.Sleep(updateInterval)
			notificationList.Refresh()
		}
	}()

	w.ShowAndRun()
}

func GetUpdates() []*fyne.Notification {
	if site == "" || username == "" || password == "" {
		return []*fyne.Notification{fyne.NewNotification("Notify", "Please configure Notify")}
	}
	data, err := BasicAuthGet(site, username, password)
	if err != nil {
		return []*fyne.Notification{fyne.NewNotification("Error", err.Error())}
	} else {
		newNotifications := Notifications{}
		err = json.Unmarshal(data, &newNotifications)
		if err != nil {
			return []*fyne.Notification{fyne.NewNotification("Error", err.Error())}
		} else {
			notifications := []*fyne.Notification{}
			for _, n := range newNotifications {
				notifications = append(notifications, fyne.NewNotification(n.Title, n.Content))
			}
			return notifications
		}
	}

}

func BasicAuthGet(url string, username string, password string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
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
