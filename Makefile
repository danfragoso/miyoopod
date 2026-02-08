.PHONY: go
go:
	CC=arm-linux-gnueabihf-gcc CGO_ENABLED=1 GOARCH=arm GOOS=linux go build -o App/MiyooPod/MiyooPod src/*.go
	mkdir -p App/MiyooPod/libs
	cp libs/libSDL2.so App/MiyooPod/libs/
	cp libs/libSDL2_mixer.so App/MiyooPod/libs/
	cp libs/libmpg123.so.0 App/MiyooPod/libs/
	cp libs/libSDL2_EGL.so App/MiyooPod/libs/
	cp libs/libSDL2_GLESv2.so App/MiyooPod/libs/
	cp libs/libSDL2_json-c.so App/MiyooPod/libs/
	cp libs/libSDL2_z.so App/MiyooPod/libs/
