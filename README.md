# Rate Limiter

Middleware de rate limiting em Go com persistência no Redis. Limita requisições por IP ou por token de API (JWT), com precedência do token sobre o IP.

## Pré-requisitos

- [Go 1.21+](https://golang.org/dl/)
- [Docker](https://docs.docker.com/get-docker/) e Docker Compose

---

## Configuração

Todas as configurações ficam no arquivo `.env` na raiz do projeto:

| Variável | Padrão | Descrição |
|---|---|---|
| `IP_REQUEST_LIMIT` | `2` | Máximo de requisições por IP por janela |
| `TOKEN_REQUEST_LIMIT` | `3` | Máximo de requisições por token por janela |
| `TIME_WINDOW` | `5s` | Janela de tempo para contagem |
| `BLOCK_TIME` | `15s` | Duração do bloqueio após exceder o limite |
| `SECRET_KEY` | — | Chave para assinar/validar os JWTs |
| `REDIS_ADDR` | `localhost:6379` | Endereço do Redis |
| `REDIS_PASSWORD` | _(vazio)_ | Senha do Redis (deixar vazio se não houver) |

---

## Como executar

### 1. Subir o Redis

```bash
docker compose up -d
```

Isso inicia o container `redisDesafio` na porta `6379`. O Redis roda **sem persistência** — cada `docker compose up` começa com estado limpo.

### 2. Iniciar o servidor

```bash
go run main.go
```

O servidor sobe na porta **8080**.

---

## Rotas

### `POST /token`
Gera um token JWT válido para usar nas requisições.

```bash
curl -s -X POST http://localhost:8080/token
```

Resposta:
```json
{ "access_token": "eyJhbGci..." }
```

### `GET /`
Endpoint protegido pelo rate limiter.

**Sem token** — limitado por IP:
```bash
curl http://localhost:8080/
```

**Com token** — limitado pelo token (limite independente do IP):
```bash
curl -H "API_KEY: <seu-token>" http://localhost:8080/
```

---

## Comportamento

### Limitação por IP
Requisições sem o header `API_KEY` são rastreadas pelo IP de origem.

### Limitação por Token
Requisições com `API_KEY: <token>` são rastreadas pelo valor do token, ignorando completamente o contador de IP.

### Regra de Ouro — Token > IP
O token tem precedência absoluta. Mesmo que o IP esteja bloqueado, requisições com um token válido dentro do seu próprio limite são aceitas.

### Resposta ao exceder o limite

```
HTTP 429 Too Many Requests

you have reached the maximum number of requests or actions allowed within a certain time frame
```

---

## Testes automatizados

Os testes são de integração: sobem contra o servidor real e o Redis. O servidor e o Redis precisam estar rodando antes de executar.

### Pré-requisitos para os testes

```bash
# Terminal 1 — Redis
docker compose up -d

# Terminal 2 — Servidor
go run main.go
```

### Executar todos os testes

```bash
go test -v -timeout 600s .
```

### Executar um teste específico

```bash
go test -v -timeout 300s -run TestIPRateLimit .
go test -v -timeout 300s -run TestTokenRateLimit .
go test -v -timeout 300s -run TestTokenPrecedenceOverIP .
go test -v -timeout 300s -run TestConcurrentRequests .
```

### Testes disponíveis

| Teste | O que valida |
|---|---|
| `TestIPRateLimit` | Limite por IP, persistência do bloqueio e desbloqueio automático |
| `TestTokenRateLimit` | Limite por token, persistência do bloqueio e desbloqueio automático |
| `TestTokenPrecedenceOverIP` | Token aceita requisições mesmo com IP bloqueado (Regra de Ouro) |
| `TestConcurrentRequests` | Corretude do limiter sob goroutines concorrentes |

Cada teste aguarda `BLOCK_TIME + 1s` no início para garantir estado limpo no Redis, então o tempo total da suíte completa é de aproximadamente **2 minutos**.

### Parâmetros dos testes

Os testes leem `IP_REQUEST_LIMIT`, `TOKEN_REQUEST_LIMIT`, `TIME_WINDOW` e `BLOCK_TIME` diretamente do `.env`. Alterar o `.env` altera o comportamento dos testes automaticamente.

---

## Arquitetura

```
.
├── main.go                          # Entrypoint — monta servidor, Redis e middleware
├── config/config.go                 # Leitura do .env via Viper
├── pkg/
│   ├── ratelimiter/
│   │   ├── interface.go             # Interface Strategy (Allow)
│   │   └── redis_ratelimiter.go    # Implementação com Redis
│   └── token/token.go              # Geração e validação de JWT
└── internal/web/
    ├── middleware/middleware.go      # Middleware HTTP do rate limiter
    └── handlers/token.go            # Handler POST /token
```

O padrão **Strategy** está em `pkg/ratelimiter/interface.go`. Para trocar o Redis por outro backend, basta implementar `Interface` e injetar a nova implementação em `main.go`.
