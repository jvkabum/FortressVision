module FortressVision

go 1.25.4

require (
	github.com/gorilla/websocket v1.5.3
	google.golang.org/protobuf v1.36.11
	gorm.io/driver/sqlite v1.6.0
	gorm.io/gorm v1.31.1
	kaijuengine.com v0.0.0-00010101000000-000000000000
)

require (
	github.com/KaijuEngine/uuid v1.0.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	github.com/tdewolff/parse/v2 v2.8.1 // indirect
	golang.design/x/clipboard v0.7.1 // indirect
	golang.org/x/exp/shiny v0.0.0-20250819193227-8b4c13bb791b // indirect
	golang.org/x/image v0.30.0 // indirect
	golang.org/x/mobile v0.0.0-20250813145510-f12310a0cfd9 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
)

replace kaijuengine.com => ../kaiju/src
