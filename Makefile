.PHONY: proto

run: build
	rm -rf _hod_
	./mortar


build:
	CGO_CFLAGS_ALLOW=.*/git.sr.ht/%7Egabe/hod/turtle go build -o mortar

clean:
	rm -rf _hod_
