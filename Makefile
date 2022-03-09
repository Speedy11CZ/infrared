test:
	go install github.com/golang/mock/mockgen@v1.6.0
	go generate ./...
	go test -race -timeout 10s ./...

dev:
	docker-compose -p infrared -f deployments/docker-compose.dev.yml up

run:
	go run -race ./cmd/infrared -config-path=./configs/config.dev.yml

test-docker:
	docker-compose -p infrared -f deployments/docker-compose.test.yml build --no-cache --force-rm
	docker-compose -p infrared -f deployments/docker-compose.test.yml up --force-recreate --remove-orphans