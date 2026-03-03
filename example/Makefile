.PHONY: up down logs proto clean

up:
	docker compose up --build -d

down:
	docker compose down -v

logs:
	docker compose logs -f

proto:
	buf generate

clean:
	docker compose down -v --rmi local
