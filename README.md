# Tell Your Story API

Backend em Go para o jogo de ice breaker "Tell Your Story".

## Stack

- Go 1.21+
- HTTP com `net/http`
- OpenAPI/Swagger embutido no binario
- Storage configuravel:
  - `memory` para desenvolvimento rapido
  - `postgres` com GORM para persistencia real
- WebSocket para sincronizacao de sala em tempo real

## Funcionalidades atuais

- criacao e consulta de salas
- entrada e saida de participantes
- inicio, pausa e avanco de rodadas
- maquina de estados de rodada: `writing -> voting -> revealed`
- submissao de historias
- votacao com prevencao de voto duplicado
- revelacao da historia vencedora
- documentacao Swagger em runtime
- eventos realtime para presenca, estado de sala, progresso de historias e progresso de votos
- autenticacao por `session_token` nas acoes mutaveis e no WebSocket

## Requisitos

- Go 1.21 ou superior
- opcional: PostgreSQL local se quiser usar `STORAGE_DRIVER=postgres`

## Configuracao

Crie o arquivo `.env` a partir do exemplo:

```powershell
Copy-Item .env.example .env
```

### Modo mais simples: memoria

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

Modo em memoria:

```powershell
Copy-Item .env.example .env
go run ./cmd/server/main.go
```

Modo com PostgreSQL ja instalado na maquina:

```powershell
Copy-Item .env.example .env
# ajuste STORAGE_DRIVER=postgres no .env
go run ./cmd/server/main.go
```

### Com Makefile

```powershell
make run
```

Se `make` nao estiver disponivel no Windows, use `go run ./cmd/server/main.go`.

## Como rodar com Docker Compose

```powershell
Copy-Item .env.example .env
docker-compose up -d --build
```

## URLs uteis

- Health check: `http://localhost:8080/health`
- Swagger UI: `http://localhost:8080/swagger/`
- OpenAPI YAML: `http://localhost:8080/swagger/openapi.yaml`
- WebSocket: `ws://localhost:8080/ws?room_code=SEU_CODIGO&user_id=SEU_USER_ID&session_token=SEU_TOKEN`

## Comandos uteis

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
- `GET /api/users/{userId}/rounds/{roundId}/vote?session_token={token}`

## Sessao

- `POST /api/rooms` e `POST /api/rooms/{code}/join` retornam `session.user_id` e `session.session_token`
- reuse esse `session_token` em `leave`, `start`, `pause`, `next-round`, `submit story`, `submit vote`, `get user vote` e no WebSocket
- o `RoomState` continua publico, mas o token nao e exposto para outros jogadores

### Realtime

- `GET /ws?room_code={code}&user_id={userId}&session_token={token}` para upgrade WebSocket

Mensagens aceitas do cliente:

- `{"type":"ping"}`
- `{"type":"room.sync"}`
- `{"type":"story.progress.request"}`
- `{"type":"vote.progress.request"}`

Eventos publicados pelo servidor:

- `connection.ready`
- `presence.joined`
- `presence.left`
- `room.state`
- `story.progress`
- `vote.progress`
- `round.revealed`
- `room.expired`
- `pong`
- `error`

## Exemplo rapido

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
cmd/server              entrypoint da aplicacao
internal/api            rotas, handlers, middleware e docs
internal/config         leitura e validacao de config
internal/database       conexao PostgreSQL
internal/domain         modelos e erros de dominio
internal/repository     repositorios memory e GORM
internal/service        regras de negocio
internal/websocket      hub realtime e eventos de sala
pkg/logger              logger estruturado
pkg/utils               utilitarios
pkg/validator           validacoes de entrada
```

## Estado atual

- a API ja roda localmente
- o Swagger ja esta disponivel em `/swagger/`
- os testes atuais cobrem a camada de servico
- o WebSocket ja publica eventos de presenca, sincronizacao de sala, progresso de historias e progresso de votos
