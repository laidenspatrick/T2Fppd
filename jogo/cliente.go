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
    clienteRPC = client 
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

    // Inicializa o jogo (com posição inicial padrão do jogoNovo)
    jogo := jogoNovo()
    if err := jogoCarregarMapa(mapaFile, &jogo); err != nil {
        panic(err)
    }

    // === INÍCIO DO BLOCO CRÍTICO: SINCRONIZAÇÃO PÓS-REGISTRO ===
    // O servidor define a posição de spawn (ex: 3, 3). O cliente deve adotar essa posição.
    if jogadorEstado, ok := resposta.EstadoAtual.Jogadores[clientID]; ok {
        jogo.PosX = jogadorEstado.X
        jogo.PosY = jogadorEstado.Y
        jogo.Vidas = jogadorEstado.Vidas // Sincroniza vidas também, apenas por garantia.
        log.Printf("Estado inicial sincronizado com o servidor: X=%d, Y=%d, Vidas=%d", jogo.PosX, jogo.PosY, jogo.Vidas)
    } else {
        log.Println("Aviso: Jogador não encontrado no estado retornado pelo servidor após o registro.")
    }
    // === FIM DO BLOCO CRÍTICO ===

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
        // interfaceDesenharJogo é chamada pelo loopAtualizacaoCliente para evitar flashes.
    }
}