.PHONY: go
go:
	CC=arm-linux-gnueabihf-gcc CGO_ENABLED=1 GOARCH=arm GOOS=linux go build -o App/MiyooPod/MiyooPod src/*.go
	cp /root/lib/libSDL2.so App/MiyooPod/
	cp /root/lib/libSDL2_mixer.so App/MiyooPod/
	cp /root/lib/libmpg123.so.0 App/MiyooPod/
	cp /root/lib/libSDL2_EGL.so App/MiyooPod/
	cp /root/lib/libSDL2_GLESv2.so App/MiyooPod/
	cp /root/lib/libSDL2_json-c.so App/MiyooPod/
	cp /root/lib/libSDL2_z.so App/MiyooPod/
