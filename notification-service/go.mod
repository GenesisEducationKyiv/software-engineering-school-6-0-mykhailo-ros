module notification-service

go 1.26.4

require (
	github-release-notifier/gen v0.0.0-00010101000000-000000000000
	github.com/joho/godotenv v1.5.1
	github.com/redis/go-redis/v9 v9.21.0
	github.com/stretchr/testify v1.11.1
	google.golang.org/grpc v1.82.0
)

require (
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rabbitmq/amqp091-go v1.10.0
	go.uber.org/atomic v1.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github-release-notifier/gen => ../gen
