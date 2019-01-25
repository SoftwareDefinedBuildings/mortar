APP?=mortar
RELEASE?=0.0.10
.PHONY: proto

run: build clean
	./mortar

container: build
	cp mortar container/.
	docker build -t mortar/$(APP):$(RELEASE) container

push: container
	docker push mortar/$(APP):$(RELEASE)

build:
	CGO_CFLAGS_ALLOW=.*/git.sr.ht/%7Egabe/hod/turtle go build -o mortar

clean:
	rm -rf _hod_
