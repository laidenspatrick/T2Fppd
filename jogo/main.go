// main.go - Loop principal do jogo
package main

import (
	"os"
	"log"
	"fmt"
	"net/rpc"
	"time"
	"math/rand"
)
// Constantes de Conexão
const (
    SERVER_ADDR = "localhost:1234" 
)
func main() {
	//DEFINIR CLIENT ID E CONECTAR RPC
    rand.Seed(time.Now().UnixNano())
    clientID = fmt.Sprintf("Jogador-%d", rand.Intn(10000))
    log.Printf("Iniciando Cliente: %s", clientID)

	// Tenta conectar ao servidor RPC
    client, err := rpc.Dial("tcp", SERVER_ADDR)
    if err != nil {
        log.Fatalf("Falha ao conectar ao servidor RPC (%s). O servidor de jogo deve estar rodando: %v", SERVER_ADDR, err)
    }
    clienteRPC = client // Define a variável global (em jogo.go)
    log.Println("Conexão RPC estabelecida.")

	// REGISTRO NO SERVIDOR
    sequence = 1 
    registroComando := Comando{
        ClientID: clientID,
        SequenceNumber: sequence,
        Acao: "register",
    }
    var resposta Resposta
    
    // Chamada RPC para registrar o jogador
    err = clienteRPC.Call("JogoServer.ExecutarComando", registroComando, &resposta)
    if err != nil || !resposta.Sucesso {
        log.Fatalf("Falha ao registrar no servidor: %v, Resposta: %s", err, resposta.Mensagem)
    }
    log.Println("Registro no servidor concluído com sucesso.")

	// Inicializa a interface (termbox)
	interfaceIniciar()
	defer interfaceFinalizar()

	// Usa "mapa.txt" como arquivo padrão ou lê o primeiro argumento
	mapaFile := "mapa.txt"
	if len(os.Args) > 1 {
		mapaFile = os.Args[1]
	}

	// Inicializa o jogo
	jogo := jogoNovo()
	if err := jogoCarregarMapa(mapaFile, &jogo); err != nil {
		panic(err)
	}

	// inicia o comportamento dos elementos 
    iniciarElementos(&jogo)

	// INICIAR LOOP DE BUSCA DE ESTADO (DEVE SER CHAMADO APÓS A INICIALIZAÇÃO DO JOGO)
    go loopAtualizacaoCliente(&jogo, clienteRPC, clientID)

	// Desenha o estado inicial do jogo
	interfaceDesenharJogo(&jogo)

	// Loop principal de entrada
	for {
		evento := interfaceLerEventoTeclado()
		if continuar := personagemExecutarAcao(evento, &jogo); !continuar {
			break
		}
		interfaceDesenharJogo(&jogo)
	}
}