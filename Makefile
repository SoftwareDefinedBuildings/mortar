APP?=mortar
RELEASE?=0.0.11
.PHONY: proto

run: build clean
	./mortar

container: build
	cp mortar containers/mortar-server
	docker build -t mortar/$(APP):$(RELEASE) containers/mortar-server

client-container:
	cd containers/pymortar-client && bash generate-ssl.sh
	docker build -t mortar/pymortar-client:$(RELEASE) containers/pymortar-client
	docker run -p 8889:8888 --name mortar -e USE_HTTPS=yes -e MORTAR_API_ADDRESS=mortardata.org:9001 -e MORTAR_API_USERNAME=$(MORTAR_API_USERNAME) -e MORTAR_API_PASSWORD=$(MORTAR_API_PASSWORD) --rm mortar/pymortar-client:$(RELEASE)

push: container client-container
	docker push mortar/$(APP):$(RELEASE)
	docker push mortar/pymortar-client:$(RELEASE)

build:
	CGO_CFLAGS_ALLOW=.*/git.sr.ht/%7Egabe/hod/turtle go build -o mortar

clean:
	rm -rf _hod_
