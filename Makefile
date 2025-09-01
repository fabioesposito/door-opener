build:
	go build -o door-opener .

clean:
	rm -f door-opener

docker-build:
	docker build -t door-opener .

docker-run:
	docker run --rm -p 8080:8080 --env-file .env door-opener

.PHONY: build clean docker-build docker-run