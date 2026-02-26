module github.com/user/alsamixer-web

go 1.23.0

require (
	github.com/fsnotify/fsnotify v1.9.0
	github.com/gen2brain/alsa v0.5.0
)

require golang.org/x/sys v0.35.0 // indirect

replace github.com/gen2brain/alsa => github.com/cbrunnkvist/alsa v0.5.1-0.20260226055441-679e6c900c3c
