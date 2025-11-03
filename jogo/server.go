package main

import (
    "fmt"
    "log"
    "net"
    "net/rpc"
    "sync"
)

// JogoServer é o tipo que implementa os métodos RPC.
type JogoServer struct {
    estado EstadoJogo
    mu     sync.Mutex 
    Jogadores map[string]EstadoJogador
}

// NovoJogoServer inicializa o servidor de jogo.
func NovoJogoServer() *JogoServer {
    return &JogoServer{
        estado: EstadoJogo{
            Jogadores: make(map[string]EstadoJogador),
        },
    }
}

// Implementação do RPC: BuscarEstado
func (s *JogoServer) BuscarEstado(comando *Comando, resposta *Resposta) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    fmt.Printf("[Servidor] REQ: %s, Cliente: %s\n", comando.Acao, comando.ClientID)
    *resposta = Resposta{
        Sucesso:  true,
        Mensagem: "Estado atual enviado.",
        EstadoAtual: s.estado,
    }
    return nil
}

// Implementação do RPC: ExecutarComando
func (s *JogoServer) ExecutarComando(comando *Comando, resposta *Resposta) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    fmt.Printf("[Servidor] REQ: %s, Cliente: %s, Seq: %d, Detalhe: %s\n", 
        comando.Acao, comando.ClientID, comando.SequenceNumber, comando.Detalhe)

    jogador, existe := s.estado.Jogadores[comando.ClientID]
    
    // 1. Garantia de Execução Única (Exactly-Once)
    if existe && comando.SequenceNumber <= jogador.UltimoComando {
        *resposta = Resposta{
            Sucesso:  true,
            Mensagem: "Comando já processado (retransmissão detectada).",
            EstadoAtual: s.estado,
        }
        return nil 
    }

    mensagemServidor := ""

    // 2. Execução do Comando e Atualização do Estado
    switch comando.Acao {
    case "register":
        if !existe {
            s.estado.Jogadores[comando.ClientID] = EstadoJogador{
                X: 3, 
                Y: 3,
                Vidas: 3,
                UltimoComando: comando.SequenceNumber,
            }
            mensagemServidor = "Jogador registrado com sucesso."
        }
        
    case "update_position":
        var newX, newY, newVidas int
        
        // Tenta ler o formato que inclui Vidas (usado após cair em armadilha, por exemplo)
        // O cliente envia: "X:%d,Y:%d;VIDAS:%d"
        _, errVidas := fmt.Sscanf(comando.Detalhe, "X:%d,Y:%d;VIDAS:%d", &newX, &newY, &newVidas)

        if errVidas == nil {
            // Sucesso na leitura de posição e vidas
            jogador.Vidas = newVidas
            jogador.X = newX
            jogador.Y = newY
            mensagemServidor = fmt.Sprintf("Posição e Vidas atualizadas: X=%d, Y=%d, Vidas=%d", newX, newY, newVidas)
            
        } else {
            // Tenta ler o formato de movimento simples (apenas posição)
            // O cliente envia: "X:%d,Y:%d"
            if _, errPos := fmt.Sscanf(comando.Detalhe, "X:%d,Y:%d", &newX, &newY); errPos == nil {
                // Sucesso na leitura da posição (vidas permanecem as mesmas)
                jogador.X = newX
                jogador.Y = newY
                mensagemServidor = fmt.Sprintf("Posição atualizada: X=%d, Y=%d", newX, newY)
            } else {
                // Falha ao ler o formato, provavelmente um erro.
                mensagemServidor = "Erro de formato no detalhe da posição. Posição não atualizada."
                break // Sai do switch, mas mantém o sequence number (pode ser reenvio)
            }
        }
        
        // Aplica o último comando processado e atualiza o estado
        jogador.UltimoComando = comando.SequenceNumber
        s.estado.Jogadores[comando.ClientID] = jogador
        
    case "interact":
        // Este comando pode ser usado para interações que não são movimento (ex: usar item)
        // Por enquanto, apenas atualizamos o sequence number
        jogador.UltimoComando = comando.SequenceNumber
        s.estado.Jogadores[comando.ClientID] = jogador
        mensagemServidor = "Interação registrada."
        
    }

    // 3. Resposta final para o cliente
    *resposta = Resposta{
        Sucesso:  true,
        Mensagem: mensagemServidor,
        EstadoAtual: s.estado,
    }
    return nil
}
// Inicia o Servidor RPC
func IniciarServidor(porta string) {
    servidor := NovoJogoServer()
    rpc.Register(servidor)

    listener, err := net.Listen("tcp", ":"+porta)
    if err != nil {
        log.Fatal("Erro ao iniciar listener:", err)
    }
    log.Println("Servidor RPC iniciado na porta:", porta)

    rpc.Accept(listener)
}