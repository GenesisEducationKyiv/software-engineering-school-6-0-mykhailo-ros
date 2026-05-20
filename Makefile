.PHONY: test-unit test-integration test-e2e

test-unit:
	go test ./...

test-integration:
	TESTCONTAINERS_RYUK_DISABLED=true go test -tags=integration -timeout=120s ./internal/integration/...

test-e2e:
	docker compose --profile e2e up --build --abort-on-container-exit --exit-code-from e2e; \
	code=$$?; docker compose --profile e2e down; exit $$code
