appname := kura-sowa

sources := $(wildcard *.go)

build = GOOS=$(1) GOARCH=$(2) go build -o build/$(1)/$(2)/$(appname)$(3)
tar = cd build && tar -cvzf $(1)_$(2).tar.gz $(appname)$(3) && rm $(appname)$(3)
zip = cd build && zip $(1)_$(2).zip $(appname)$(3) && rm $(appname)$(3)

.PHONY: all windows darwin linux clean

all: windows darwin linux

install:
	go install -v
clean:
	rm -rf build/

##### LINUX BUILDS #####
linux: linux-arm linux-arm64 linux-386 linux-amd64

linux-386: $(sources)
	$(call build,linux,386,)
	# $(call tar,linux,386)

linux-amd64: $(sources)
	$(call build,linux,amd64,)
	# $(call tar,linux,amd64)

linux-arm: $(sources)
	$(call build,linux,arm,)
	# $(call tar,linux,arm)

linux-arm64: $(sources)
	$(call build,linux,arm64,)
	# $(call tar,linux,arm64)

##### DARWIN (MAC) BUILDS #####
darwin: $(sources)
	$(call build,darwin,amd64,)
	# $(call tar,darwin,amd64)

build/darwin_amd64.tar.gz: $(sources)
	$(call build,darwin,amd64,)
	# $(call tar,darwin,amd64)

##### WINDOWS BUILDS #####
windows: windows-386 windows-amd64

windows-386: $(sources)
	$(call build,windows,386,.exe)
	# $(call zip,windows,386,.exe)

windows-amd64: $(sources)
	$(call build,windows,amd64,.exe)
	# $(call zip,windows,amd64,.exe)