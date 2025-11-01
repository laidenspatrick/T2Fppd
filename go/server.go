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
    estado Jogo
    mu     sync.Mutex 
}

// NovoJogoServer inicializa o servidor de jogo.
func NovoJogoServer() *JogoServer {
    return &JogoServer{
        estado: Jogo{
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

    fmt.Printf("[Servidor] REQ: %s, Cliente: %s, Seq: %d\n", comando.Acao, comando.ClientID, comando.SequenceNumber)

    jogador, existe := s.estado.Jogadores[comando.ClientID]
    if existe && comando.SequenceNumber <= jogador.UltimoComando {
        *resposta = Resposta{
            Sucesso:  true,
            Mensagem: "Comando já processado (retransmissão detectada).",
            EstadoAtual: s.estado,
        }
        return nil // Evita reexecução
    }

    // 2. Execução do Comando e Atualização do Estado
    switch comando.Acao {
    case "register":
        if !existe {
            s.estado.Jogadores[comando.ClientID] = EstadoJogador{
                X:               1, // Posição inicial
                Y:               1,
                Vidas:           3,
                UltimoComando: comando.SequenceNumber,
            }
            resposta.Mensagem = "Jogador registrado com sucesso."
        }
    case "update_position":
        jogador.UltimoComando = comando.SequenceNumber
        s.estado.Jogadores[comando.ClientID] = jogador
        resposta.Mensagem = "Posição atualizada."

    case "interact":
        jogador.UltimoComando = comando.SequenceNumber
        s.estado.Jogadores[comando.ClientID] = jogador
        resposta.Mensagem = "Interação registrada."
    }

    *resposta = Resposta{
        Sucesso:  true,
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