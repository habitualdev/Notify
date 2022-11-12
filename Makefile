build:
	cd client && fyne-cross android -app-id com.notify.client
	cd server && GOOS=linux go build -o server -ldflags "-s -w" -trimpath .
	cd server && GOOS=windows go build -o server.exe -ldflags "-s -w" -trimpath .
	mkdir -p build && mv server/server* build/ && mv client/fyne-cross/dist/android/client.apk build
	rm -rf client/fyne-cross

install-server:
	cd server && GOOS=linux go build -o server -ldflags "-s -w" -trimpath .
	cp server/template.yaml ~/.local/bin/notify-config.yaml
	mv server/server ~/.local/bin/notify-server