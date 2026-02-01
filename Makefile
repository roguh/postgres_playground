.PHONY: up down reset migrate generate seed bench clean

# Start everything
up:
	docker-compose up -d
	@echo "Waiting for PostgreSQL..."
	@sleep 3
	@echo "PostgreSQL ready at localhost:5432"
	@echo "pgAdmin at http://localhost:5050"

# Stop everything
down:
	docker-compose down

# Full reset
reset: down
	docker-compose down -v
	docker-compose up -d
	@sleep 3
	make migrate

# Run migrations
migrate:
	@echo "Running migrations..."
	go run cmd/migrate/main.go up

# Generate sqlc code
generate:
	sqlc generate

# Seed database
seed:
	go run cmd/seed/main.go

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Clean generated files
clean:
	rm -rf sqlc/
	docker-compose down -v
