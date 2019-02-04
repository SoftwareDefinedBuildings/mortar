APP?=mortar
RELEASE?=0.0.12-alpha0
MORTAR_REPOSITORY?=https://github.com/SoftwareDefinedBuildings/mortar-analytics
.PHONY: proto frontend


container: build
	cp mortar containers/mortar-server
	docker build -t mortar/$(APP):$(RELEASE) containers/mortar-server
	docker build -t mortar/$(APP):latest containers/mortar-server

client-container:
	docker build -t mortar/pymortar-client:$(RELEASE) containers/pymortar-client
	docker build -t mortar/pymortar-client:latest containers/pymortar-client

frontend-container:
	mkdocs build
	mkdir -p containers/frontend/static/site
	mkdir -p containers/frontend/static/login
	cp -r site/* containers/frontend/static/site/
	cp -r frontend/static/login/index.html containers/frontend/static/login/index.html
	go build -o containers/frontend/exec-frontend ./frontend
	docker build -t mortar/frontend:latest containers/frontend

mortar-analytics:
	git clone $(MORTAR_REPOSITORY) mortar-analytics

frontend:
	mkdocs build
	cp -r site/ frontend/static/
	go build -o frontend/exec-frontend ./frontend
	cd frontend && ./exec-frontend

run: build clean
	./mortar

run-client: client-container mortar-analytics
	bash containers/pymortar-client/generate-ssl.sh
	docker run -p 8889:8888 --name mortar -v `pwd`/mortar-analytics:/home/jovyan/mortar-analytics -e USE_HTTPS=yes -e MORTAR_API_ADDRESS=mortardata.org:9001 -e MORTAR_API_USERNAME=$(MORTAR_API_USERNAME) -e MORTAR_API_PASSWORD=$(MORTAR_API_PASSWORD) -v `pwd`/certs:/certs --rm mortar/pymortar-client:$(RELEASE)

run-frontend: frontend-container
	docker run -p 3000:3000 --name mortar-frontend --rm mortar/frontend:latest

push: container client-container
	docker push mortar/$(APP):$(RELEASE)
	docker push mortar/$(APP):latest
	docker push mortar/pymortar-client:$(RELEASE)
	docker push mortar/pymortar-client:latest
	docker push mortar/frontend:latest

push-mortar: container
	docker push mortar/$(APP):$(RELEASE)

build:
	CGO_CFLAGS_ALLOW=.*/git.sr.ht/%7Egabe/hod/turtle go build -o mortar

clean:
	rm -rf _hod_
