# Tell Your Story API

Backend em Go para o jogo de ice breaker "Tell Your Story".

## Stack

- Go 1.21+
- HTTP com `net/http`
- OpenAPI/Swagger embutido no binário
- Storage configurável:
  - `memory` para desenvolvimento rápido
  - `postgres` com GORM para persistência real

## Funcionalidades atuais

- criação e consulta de salas
- entrada e saída de participantes
- início, pausa e avanço de rodadas
- submissão de histórias
- votação com prevenção de voto duplicado
- revelação da história vencedora
- documentação Swagger em runtime

## Requisitos

- Go 1.21 ou superior
- opcional: PostgreSQL local se quiser usar `STORAGE_DRIVER=postgres`

## Configuração

Crie o arquivo `.env` a partir do exemplo:

```powershell
Copy-Item .env.example .env
```

### Modo mais simples: memória

Use estes valores no `.env`:

```env
PORT=8080
ENV=development
STORAGE_DRIVER=memory
```

### Modo com PostgreSQL local

Use estes valores no `.env`:

```env
PORT=8080
ENV=development
STORAGE_DRIVER=postgres
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=tell_your_story
DB_SSLMODE=disable
```

## Como rodar localmente

### Sem Docker Compose

Modo em memória:

```powershell
Copy-Item .env.example .env
go run ./cmd/server/main.go
```

Modo com PostgreSQL já instalado na máquina:

```powershell
Copy-Item .env.example .env
# ajuste STORAGE_DRIVER=postgres no .env
go run ./cmd/server/main.go
```

### Com Makefile

```powershell
make run
```

Se `make` não estiver disponível no Windows, use `go run ./cmd/server/main.go`.

## Como rodar com Docker Compose

```powershell
Copy-Item .env.example .env
docker-compose up -d --build
```

## URLs úteis

- Health check: `http://localhost:8080/health`
- Swagger UI: `http://localhost:8080/swagger/`
- OpenAPI YAML: `http://localhost:8080/swagger/openapi.yaml`

## Comandos úteis

```powershell
go test ./...
go vet ./...
make build
make test
make swagger-url
```

## Endpoints principais

### Health

- `GET /health`

### Rooms

- `POST /api/rooms`
- `GET /api/rooms/{code}`
- `POST /api/rooms/{code}/join`
- `POST /api/rooms/{code}/leave`
- `POST /api/rooms/{code}/start`
- `POST /api/rooms/{code}/pause`
- `POST /api/rooms/{code}/next-round`

### Stories

- `POST /api/stories`
- `GET /api/rounds/{roundId}/stories`

### Votes

- `POST /api/votes`
- `GET /api/rounds/{roundId}/votes`
- `GET /api/rounds/{roundId}/top-story`
- `GET /api/users/{userId}/rounds/{roundId}/vote`

## Exemplo rápido

Criar sala:

```powershell
$body = @{
  host_nickname = "Ricardo"
  host_avatar_url = "fox"
  max_rounds = 3
  time_per_round = 120
} | ConvertTo-Json

Invoke-RestMethod -Method Post `
  -Uri "http://localhost:8080/api/rooms" `
  -ContentType "application/json" `
  -Body $body
```

## Estrutura do projeto

```text
cmd/server              entrypoint da aplicação
internal/api            rotas, handlers, middleware e docs
internal/config         leitura e validação de config
internal/database       conexão PostgreSQL
internal/domain         modelos e erros de domínio
internal/repository     repositórios memory e GORM
internal/service        regras de negócio
pkg/logger              logger estruturado
pkg/utils               utilitários
pkg/validator           validações de entrada
```

## Estado atual

- a API já roda localmente
- o Swagger já está disponível em `/swagger/`
- os testes atuais cobrem a camada de serviço
- WebSocket ainda não foi implementado

