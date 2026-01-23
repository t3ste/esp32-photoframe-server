.PHONY: format build run

format:
	@echo "Formatting Go code..."
	gofmt -w server/
	@echo "Formatting Frontend code..."
	cd frontend && npm run format

build:
	docker build -t photoframe-server .

run:
	docker rm -f photoframe-server || true
	docker run -d -p 9607:9607 -v "$(PWD)/data:/data" --name photoframe-server photoframe-server
