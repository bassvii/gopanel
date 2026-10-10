.PHONY: build web run clean

build: web
	go build -o bin/gopanel ./cmd/gopanel

web:
	cd web && npm run build

run: build
	./bin/gopanel run --db /tmp/gp.db

clean:
	rm -rf bin web/dist
