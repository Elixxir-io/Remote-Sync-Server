.PHONY: update master release update_master update_release build clean binary version

version:
	go run main.go generate
	mv version_vars.go cmd/version_vars.go

clean:
	go mod tidy
	go mod vendor -e

update:
	-GOFLAGS="" go get all

build:
	go build ./...

update_release:
	GOFLAGS="" go get gitlab.com/xx_network/primitives@release
	GOFLAGS="" go get gitlab.com/elixxir/comms@release

update_master:
	GOFLAGS="" go get gitlab.com/xx_network/primitives@master
	GOFLAGS="" go get gitlab.com/elixxir/comms@release

binary:
	go build -ldflags '-w -s' -trimpath -o remoteSyncServer main.go

master: update_master clean build

release: update_release clean build
