.PHONY: setup dev dev-backend dev-frontend test e2e fmt build clean lint lint-fix ci-local hooks-install audit

setup:
	cd backend && go mod download
	cd frontend && npm install
	@$(MAKE) hooks-install

dev:
	@$(MAKE) -j2 dev-backend dev-frontend

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

test:
	cd backend && go test ./...
	cd frontend && npm test -- --run

e2e:
	cd frontend && npm run e2e

fmt:
	cd backend && gofmt -s -w . && goimports -w .
	cd frontend && npm run format

build:
	cd backend && go build -o bin/server ./cmd/server
	cd frontend && npm run build

clean:
	rm -rf backend/bin frontend/build frontend/.svelte-kit

lint:
	cd backend && golangci-lint run
	cd backend && go vet ./...
	cd frontend && npx eslint . --max-warnings 0
	cd frontend && npx prettier --check .
	cd frontend && npx stylelint '**/*.{css,svelte}' --max-warnings 0
	cd frontend && npx svelte-check --tsconfig ./tsconfig.json --threshold error

lint-fix:
	cd backend && golangci-lint run --fix
	cd frontend && npx eslint . --fix
	cd frontend && npx stylelint '**/*.{css,svelte}' --fix
	cd frontend && npx prettier --write .

audit:
	cd frontend && npm audit --omit=dev --audit-level=high

ci-local:
	@$(MAKE) lint test audit

hooks-install:
	@command -v lefthook >/dev/null 2>&1 || go install github.com/evilmartians/lefthook@v1.7.22
	lefthook install
