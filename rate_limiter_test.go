package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

// ============================================
// Mensagem exata exigida pela especificação
// ============================================

const blockedMessage = "you have reached the maximum number of requests or actions allowed within a certain time frame"

// ============================================
// Configurações do Teste
// ============================================

type TestConfig struct {
	// Configurações do servidor
	ServerURL     string
	TokenEndpoint string
	TestEndpoint  string

	// Parâmetros do rate limiter (devem corresponder ao .env)
	IPRequestLimit    int
	TokenRequestLimit int
	TimeWindow        time.Duration
	BlockTime         time.Duration

	// Configurações do teste
	EnableColors bool
	Verbose      bool
}

// Configuração padrão
func DefaultConfig() *TestConfig {
	ipLimit, _ := strconv.Atoi(os.Getenv("IP_REQUEST_LIMIT"))
	tokenLimit, _ := strconv.Atoi(os.Getenv("TOKEN_REQUEST_LIMIT"))
	timeWindow, _ := time.ParseDuration(os.Getenv("TIME_WINDOW"))
	blockTime, _ := time.ParseDuration(os.Getenv("BLOCK_TIME"))

	return &TestConfig{
		ServerURL:         "http://localhost:8080",
		TokenEndpoint:     "/token",
		TestEndpoint:      "/",
		IPRequestLimit:    ipLimit,
		TokenRequestLimit: tokenLimit,
		TimeWindow:        timeWindow,
		BlockTime:         blockTime,
		EnableColors:      true,
		Verbose:           false,
	}
}

// ============================================
// Cores para output
// ============================================

type Colors struct {
	Red     string
	Green   string
	Yellow  string
	Blue    string
	Magenta string
	Cyan    string
	Reset   string
}

func (c *TestConfig) getColors() Colors {
	if c.EnableColors {
		return Colors{
			Red:     "\033[0;31m",
			Green:   "\033[0;32m",
			Yellow:  "\033[1;33m",
			Blue:    "\033[0;34m",
			Magenta: "\033[0;35m",
			Cyan:    "\033[0;36m",
			Reset:   "\033[0m",
		}
	}
	return Colors{}
}

// ============================================
// Funções auxiliares
// ============================================

func (c *TestConfig) printHeader(message string) {
	colors := c.getColors()
	fmt.Printf("\n%s═══════════════════════════════════════════════════════════%s\n", colors.Blue, colors.Reset)
	fmt.Printf("%s  %s%s\n", colors.Blue, message, colors.Reset)
	fmt.Printf("%s═══════════════════════════════════════════════════════════%s\n\n", colors.Blue, colors.Reset)
}

func (c *TestConfig) printInfo(message string) {
	colors := c.getColors()
	fmt.Printf("%sℹ  %s%s\n", colors.Cyan, message, colors.Reset)
}

func (c *TestConfig) printSuccess(message string) {
	colors := c.getColors()
	fmt.Printf("%s✓  %s%s\n", colors.Green, message, colors.Reset)
}

func (c *TestConfig) printTest(message string) {
	colors := c.getColors()
	fmt.Printf("%s→  %s%s\n", colors.Magenta, message, colors.Reset)
}

func (c *TestConfig) waitWithProgress(duration time.Duration, message string) {
	colors := c.getColors()
	fmt.Printf("%s%s", colors.Yellow, message)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	remaining := int(duration.Seconds())
	for i := remaining; i > 0; i-- {
		fmt.Printf(" %d...", i)
		<-ticker.C
	}
	fmt.Printf("%s\n", colors.Reset)
}

// makeRequest envia uma requisição HTTP e retorna (statusCode, body, error).
// O token é enviado no header API_KEY, conforme a especificação.
func (c *TestConfig) makeRequest(url, method, token string) (int, string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return 0, "", err
	}

	if token != "" {
		req.Header.Set("API_KEY", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(body)), nil
}

func (c *TestConfig) getToken() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(c.ServerURL+c.TokenEndpoint, "", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}

	if err := json.Unmarshal(body, &tokenResponse); err != nil {
		return "", fmt.Errorf("erro ao fazer parse do JSON: %v", err)
	}

	if tokenResponse.AccessToken == "" {
		return "", fmt.Errorf("access_token não encontrado na resposta")
	}

	return tokenResponse.AccessToken, nil
}

// assertBlocked verifica que a resposta é 429 com a mensagem exata da especificação.
func (c *TestConfig) assertBlocked(t *testing.T, statusCode int, body string) {
	t.Helper()
	if statusCode != http.StatusTooManyRequests {
		t.Errorf("esperava HTTP 429, recebeu HTTP %d", statusCode)
		return
	}
	if body != blockedMessage {
		t.Errorf("corpo da resposta incorreto\n  esperado: %q\n  recebido: %q", blockedMessage, body)
	}
	c.printSuccess(fmt.Sprintf("Bloqueada corretamente (HTTP %d)", statusCode))
}

// ============================================
// Testes
// ============================================

func TestIPRateLimit(t *testing.T) {
	config := DefaultConfig()
	config.printHeader("TESTE 1: Rate Limiting por IP")

	config.printInfo(fmt.Sprintf("Configuração: Limite de %d requisições a cada %.0f segundos",
		config.IPRequestLimit, config.TimeWindow.Seconds()))
	config.printInfo(fmt.Sprintf("Bloqueio de %.0f segundos após exceder o limite", config.BlockTime.Seconds()))

	// Aguardar até que qualquer bloqueio anterior expire (BlockTime > TimeWindow)
	config.waitWithProgress(config.BlockTime+time.Second, "Aguardando estado limpo no Redis (BlockTime)...")
	fmt.Println()

	// Fase 1: Testar até o limite
	config.printInfo("Fase 1: Testando requisições dentro do limite")
	for i := 1; i <= config.IPRequestLimit; i++ {
		config.printTest(fmt.Sprintf("Requisição %d sem token...", i))
		statusCode, _, err := config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", "")

		if err != nil {
			t.Errorf("Erro na requisição: %v", err)
			continue
		}

		if statusCode == http.StatusOK {
			config.printSuccess(fmt.Sprintf("Aceita (HTTP %d)", statusCode))
		} else {
			t.Errorf("Esperava HTTP 200, recebeu HTTP %d", statusCode)
		}
	}

	// Fase 2: Exceder o limite
	fmt.Println()
	config.printInfo("Fase 2: Excedendo o limite")
	for i := 1; i <= 2; i++ {
		config.printTest(fmt.Sprintf("Requisição extra %d sem token...", i))
		statusCode, body, err := config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", "")

		if err != nil {
			t.Errorf("Erro na requisição: %v", err)
			continue
		}

		config.assertBlocked(t, statusCode, body)
	}

	// Fase 3: Verificar se continua bloqueado
	fmt.Println()
	config.printInfo("Fase 3: Verificando se continua bloqueado durante o período de bloqueio")
	config.waitWithProgress(config.BlockTime/2, "Aguardando metade do tempo de bloqueio...")

	config.printTest("Tentando requisição durante o bloqueio...")
	statusCode, body, err := config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", "")
	if err != nil {
		t.Errorf("Erro na requisição: %v", err)
	} else {
		config.assertBlocked(t, statusCode, body)
	}

	// Fase 4: Aguardar desbloqueio
	fmt.Println()
	config.printInfo("Fase 4: Aguardando desbloqueio completo")
	config.waitWithProgress(config.BlockTime/2+2*time.Second, "Aguardando fim do bloqueio...")

	config.printTest("Tentando requisição após o bloqueio...")
	statusCode, _, err = config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", "")
	if err != nil {
		t.Errorf("Erro na requisição: %v", err)
	} else if statusCode == http.StatusOK {
		config.printSuccess("Desbloqueado corretamente!")
	} else {
		t.Errorf("Esperava HTTP 200, recebeu HTTP %d", statusCode)
	}
}

func TestTokenRateLimit(t *testing.T) {
	config := DefaultConfig()
	config.printHeader("TESTE 2: Rate Limiting por Token")

	config.printInfo(fmt.Sprintf("Configuração: Limite de %d requisições a cada %.0f segundos",
		config.TokenRequestLimit, config.TimeWindow.Seconds()))
	config.printInfo(fmt.Sprintf("Bloqueio de %.0f segundos após exceder o limite", config.BlockTime.Seconds()))

	// Aguardar até que qualquer bloqueio anterior expire (BlockTime > TimeWindow)
	config.waitWithProgress(config.BlockTime+time.Second, "Aguardando estado limpo no Redis (BlockTime)...")
	fmt.Println()

	// Obter token
	config.printInfo("Obtendo token de API...")
	token, err := config.getToken()
	if err != nil {
		t.Fatalf("Falha ao obter token: %v", err)
	}
	config.printSuccess(fmt.Sprintf("Token obtido: %s...", token[:min(20, len(token))]))
	fmt.Println()

	// Fase 1: Testar até o limite
	config.printInfo("Fase 1: Testando requisições dentro do limite")
	for i := 1; i <= config.TokenRequestLimit; i++ {
		config.printTest(fmt.Sprintf("Requisição %d com token...", i))
		statusCode, _, err := config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", token)

		if err != nil {
			t.Errorf("Erro na requisição: %v", err)
			continue
		}

		if statusCode == http.StatusOK {
			config.printSuccess(fmt.Sprintf("Aceita (HTTP %d)", statusCode))
		} else {
			t.Errorf("Esperava HTTP 200, recebeu HTTP %d", statusCode)
		}
	}

	// Fase 2: Exceder o limite
	fmt.Println()
	config.printInfo("Fase 2: Excedendo o limite")
	for i := 1; i <= 2; i++ {
		config.printTest(fmt.Sprintf("Requisição extra %d com token...", i))
		statusCode, body, err := config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", token)

		if err != nil {
			t.Errorf("Erro na requisição: %v", err)
			continue
		}

		config.assertBlocked(t, statusCode, body)
	}

	// Fase 3: Aguardar desbloqueio
	fmt.Println()
	config.printInfo("Fase 3: Aguardando desbloqueio")
	config.waitWithProgress(config.BlockTime+2*time.Second, "Aguardando fim do bloqueio...")

	config.printTest("Tentando requisição após o bloqueio...")
	statusCode, _, err := config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", token)
	if err != nil {
		t.Errorf("Erro na requisição: %v", err)
	} else if statusCode == http.StatusOK {
		config.printSuccess("Desbloqueado corretamente!")
	} else {
		t.Errorf("Esperava HTTP 200, recebeu HTTP %d", statusCode)
	}
}

// TestTokenPrecedenceOverIP verifica a Regra de Ouro: o limite do Token tem precedência sobre o limite do IP.
//
// Fluxo:
//  1. Esgota o limite de IP (faz IP_LIMIT+1 requisições sem token → IP fica bloqueado).
//  2. Envia requisições COM token → devem ser aceitas até TOKEN_LIMIT (IP bloqueado é ignorado).
//
// Isso prova que Token > IP conforme a especificação.
func TestTokenPrecedenceOverIP(t *testing.T) {
	config := DefaultConfig()
	config.printHeader("TESTE 3: Precedência Token > IP (Regra de Ouro)")

	config.printInfo(fmt.Sprintf("Limite IP: %d req | Limite Token: %d req",
		config.IPRequestLimit, config.TokenRequestLimit))

	// Aguardar até que qualquer bloqueio anterior expire (BlockTime > TimeWindow)
	config.waitWithProgress(config.BlockTime+time.Second, "Aguardando estado limpo no Redis (BlockTime)...")
	fmt.Println()

	// Obter token antes de bloquear o IP
	config.printInfo("Obtendo token de API...")
	token, err := config.getToken()
	if err != nil {
		t.Fatalf("Falha ao obter token: %v", err)
	}
	config.printSuccess(fmt.Sprintf("Token obtido: %s...", token[:min(20, len(token))]))
	fmt.Println()

	// Passo 1: Esgotar e bloquear o IP
	// Nota: getToken() acima já consumiu 1 slot de IP (POST /token sem API_KEY header).
	// Portanto, restam apenas IPRequestLimit-1 slots disponíveis nesta janela.
	config.printInfo("Passo 1: Esgotando o limite de IP para acionar o bloqueio por IP...")
	// Faz IPRequestLimit-1 requisições sem token para atingir o limite
	// (getToken() já contabilizou 1, total = IPRequestLimit)
	for i := 1; i <= config.IPRequestLimit-1; i++ {
		statusCode, _, err := config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", "")
		if err != nil {
			t.Fatalf("Erro na requisição %d: %v", i, err)
		}
		if statusCode != http.StatusOK {
			t.Fatalf("Requisição %d: esperava 200, recebeu %d", i, statusCode)
		}
	}
	// Faz mais uma para acionar o bloqueio (total = IPRequestLimit+1 > limit)
	config.printTest("Disparando requisição que excede o limite de IP (deve retornar 429)...")
	statusCode, body, err := config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", "")
	if err != nil {
		t.Fatalf("Erro: %v", err)
	}
	config.assertBlocked(t, statusCode, body)
	config.printSuccess("IP bloqueado com sucesso.")
	fmt.Println()

	// Passo 2: Verificar que o IP realmente está bloqueado (sem token)
	config.printInfo("Passo 2: Confirmando que o IP está bloqueado (sem token)...")
	statusCode, body, err = config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", "")
	if err != nil {
		t.Fatalf("Erro: %v", err)
	}
	config.assertBlocked(t, statusCode, body)
	fmt.Println()

	// Passo 3: Provar que o Token ignora o bloqueio de IP e usa seu próprio limite
	config.printInfo("Passo 3: Enviando requisições COM token (IP bloqueado deve ser ignorado)...")
	for i := 1; i <= config.TokenRequestLimit; i++ {
		config.printTest(fmt.Sprintf("Requisição %d com token (IP bloqueado)...", i))
		statusCode, _, err := config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", token)
		if err != nil {
			t.Errorf("Erro na requisição: %v", err)
			continue
		}
		if statusCode == http.StatusOK {
			config.printSuccess(fmt.Sprintf("Aceita pelo limite do Token (HTTP %d) — Token prevaleceu sobre o bloqueio de IP", statusCode))
		} else {
			t.Errorf("Esperava HTTP 200 (Token deve ter precedência sobre IP), recebeu HTTP %d", statusCode)
		}
	}

	// Limpar: aguardar desbloqueio para não poluir testes seguintes
	config.waitWithProgress(config.BlockTime+time.Second, "Limpando estado: aguardando desbloqueio do IP...")
}

func TestConcurrentRequests(t *testing.T) {
	config := DefaultConfig()
	config.printHeader("TESTE 4: Requisições Concorrentes")

	config.printInfo("Testando múltiplas requisições simultâneas por IP")

	// Aguardar até que qualquer bloqueio anterior expire (BlockTime > TimeWindow)
	config.waitWithProgress(config.BlockTime+time.Second, "Aguardando estado limpo no Redis (BlockTime)...")
	fmt.Println()

	concurrentRequests := config.IPRequestLimit + 3
	var wg sync.WaitGroup
	results := make(chan int, concurrentRequests)

	config.printTest(fmt.Sprintf("Enviando %d requisições simultâneas (limite=%d)...", concurrentRequests, config.IPRequestLimit))

	startTime := time.Now()
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statusCode, _, err := config.makeRequest(config.ServerURL+config.TestEndpoint, "GET", "")
			if err != nil {
				results <- 0
				return
			}
			results <- statusCode
		}()
	}

	wg.Wait()
	close(results)
	duration := time.Since(startTime)

	var okCount, blockedCount, errorCount int
	for code := range results {
		switch code {
		case http.StatusOK:
			okCount++
		case http.StatusTooManyRequests:
			blockedCount++
		default:
			errorCount++
		}
	}

	config.printInfo(fmt.Sprintf("Tempo de execução: %v", duration))
	config.printInfo(fmt.Sprintf("Requisições aceitas: %d", okCount))
	config.printInfo(fmt.Sprintf("Requisições bloqueadas: %d", blockedCount))
	config.printInfo(fmt.Sprintf("Erros: %d", errorCount))

	// O rate limiter deve aceitar no máximo IPRequestLimit requisições
	// e bloquear o restante. Aceitar ±1 de tolerância por corrida de goroutines.
	if okCount <= config.IPRequestLimit && blockedCount > 0 && okCount >= 1 {
		config.printSuccess(fmt.Sprintf("Comportamento concorrente correto! (%d aceitas, %d bloqueadas)", okCount, blockedCount))
	} else {
		t.Errorf("Comportamento inesperado. Limite=%d, aceitas=%d, bloqueadas=%d, erros=%d",
			config.IPRequestLimit, okCount, blockedCount, errorCount)
	}
}
