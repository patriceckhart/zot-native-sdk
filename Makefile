GOMOBILE ?= $(shell go env GOPATH)/bin/gomobile
HOST_OS := $(shell go env GOOS)
HOST_GOARCH := $(shell go env GOARCH)
HOST_ARCH := $(HOST_GOARCH)
ifeq ($(HOST_GOARCH),amd64)
HOST_ARCH := x64
endif
HOST_EXT :=
ifeq ($(HOST_OS),windows)
HOST_EXT := .exe
endif

.PHONY: test sidecar gateway desktop desktop-all apple android stage

test:
	go test ./...

sidecar:
	mkdir -p binaries
	go build -o binaries/zot-bridge$(HOST_EXT) ./cmd/zot-bridge

gateway:
	mkdir -p binaries
	go build -o binaries/zot-gateway$(HOST_EXT) ./cmd/zot-gateway

desktop: sidecar
	mkdir -p packages/electron/bin
	cp binaries/zot-bridge$(HOST_EXT) packages/electron/bin/zot-bridge-$(HOST_OS)-$(HOST_ARCH)$(HOST_EXT)

desktop-all:
	mkdir -p packages/electron/bin
	GOOS=darwin GOARCH=arm64 go build -o packages/electron/bin/zot-bridge-darwin-arm64 ./cmd/zot-bridge
	GOOS=darwin GOARCH=amd64 go build -o packages/electron/bin/zot-bridge-darwin-x64 ./cmd/zot-bridge
	GOOS=linux GOARCH=arm64 go build -o packages/electron/bin/zot-bridge-linux-arm64 ./cmd/zot-bridge
	GOOS=linux GOARCH=amd64 go build -o packages/electron/bin/zot-bridge-linux-x64 ./cmd/zot-bridge
	GOOS=windows GOARCH=arm64 go build -o packages/electron/bin/zot-bridge-windows-arm64.exe ./cmd/zot-bridge
	GOOS=windows GOARCH=amd64 go build -o packages/electron/bin/zot-bridge-windows-x64.exe ./cmd/zot-bridge

apple:
	mkdir -p artifacts
	PATH="$(shell go env GOPATH)/bin:$$PATH" $(GOMOBILE) bind -target=ios,iossimulator,macos -iosversion 15.0 -macosversion 12.0 -o artifacts/Zot.xcframework ./bridge
	mkdir -p artifacts/mobile
	PATH="$(shell go env GOPATH)/bin:$$PATH" $(GOMOBILE) bind -target=ios,iossimulator -iosversion 15.0 -o artifacts/mobile/Zot.xcframework ./bridge

android:
	mkdir -p artifacts
	PATH="$(shell go env GOPATH)/bin:$$PATH" $(GOMOBILE) bind -target=android -androidapi 24 -o artifacts/zot.aar ./bridge

stage: apple android desktop-all
	rm -rf apple/Zot.xcframework packages/react-native/ios/Zot.xcframework packages/flutter/ios/Zot.xcframework packages/flutter/ios/zot_native/Zot.xcframework
	cp -R artifacts/Zot.xcframework apple/Zot.xcframework
	cp -R artifacts/Zot.xcframework packages/react-native/ios/Zot.xcframework
	cp -R artifacts/mobile/Zot.xcframework packages/flutter/ios/zot_native/Zot.xcframework
	cp packages/flutter/ios/Classes/ZotNativePlugin.swift packages/flutter/ios/zot_native/Sources/zot_native/ZotNativePlugin.swift
	rm -rf artifacts/android-unpacked packages/react-native/android/src/main/jniLibs packages/flutter/android/src/main/jniLibs
	mkdir -p artifacts/android-unpacked packages/react-native/android/libs packages/flutter/android/libs
	unzip -q artifacts/zot.aar classes.jar 'jni/*' -d artifacts/android-unpacked
	cp artifacts/android-unpacked/classes.jar packages/react-native/android/libs/zot.jar
	cp artifacts/android-unpacked/classes.jar packages/flutter/android/libs/zot.jar
	cp -R artifacts/android-unpacked/jni packages/react-native/android/src/main/jniLibs
	cp -R artifacts/android-unpacked/jni packages/flutter/android/src/main/jniLibs
