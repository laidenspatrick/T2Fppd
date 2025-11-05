package main

import (
	"fmt"
	"time"
)

// Atualiza a posição do personagem com base na tecla pressionada (WASD)
func jogoMoverPersonagem(jogo *Jogo, dx, dy int) {
	nx, ny := jogo.PosX+dx, jogo.PosY+dy
	// restaura célula atual sob o jogador
	jogo.Mapa[jogo.PosY][jogo.PosX] = jogo.UltimoVisitado
	// captura o elemento sob o destino
	jogo.UltimoVisitado = jogo.Mapa[ny][nx]
	// atualiza posição do jogador
	jogo.PosX, jogo.PosY = nx, ny
}

// Define o que ocorre quando o jogador pressiona a tecla de interação
func personagemInteragir(jogo *Jogo) {
	jogo.StatusMsg = fmt.Sprintf("Interagindo em (%d, %d)", jogo.PosX, jogo.PosY)
}

// personagemExecutarAcao processa o evento do teclado e envia o comando ao servidor.
func personagemExecutarAcao(ev EventoTeclado, jogo *Jogo) bool {
    // Lógica de Reinício
    if ev.Tipo == "mover" && (ev.Tecla == 'r' || ev.Tecla == 'R') {
        withMapaLock(func() {
            if jogo.GameOver {
                if err := jogoReiniciar(jogo); err != nil {
                    jogo.StatusMsg = "Falha ao reiniciar."
                }
            }
        })
        return true
    }
    if clienteRPC == nil {
        return true
    }

    var comando Comando
    
    // O Sequence Number deve ser incrementado ANTES de ser usado no comando.
    sequence++ 
    
    // Variáveis para armazenar o estado final e o detalhe
    newX, newY := jogo.PosX, jogo.PosY // Inicia com a posição atual
    acaoServidor := ""
    detalheServidor := ""
    
    // 1. Criação e Lógica do Comando RPC
    switch ev.Tipo {
    case "sair":
        return false
        
    case "mover":
        dx, dy := 0, 0
        switch ev.Tecla {
        case 'w', 'W':
            dy = -1
        case 's', 'S':
            dy = 1
        case 'a', 'A':
            dx = -1
        case 'd', 'D':
            dx = 1
        }
        
        // Posição de destino após o input
        newX, newY = jogo.PosX+dx, jogo.PosY+dy
        
        // 1a. Validação de Movimento (Limites e Paredes)
        if jogoPodeMoverPara(jogo, newX, newY) {
            
            // 1b. Interação com Elementos (Armadilhas e Portais)
            elemento := jogo.Mapa[newY][newX].simbolo

            if elemento == portal.Elemento.simbolo {
                // LÓGICA DE PORTAL: Teletransporte imediato
                destinoX, destinoY := jogoEncontrarSaida(jogo.Mapa, newX, newY)
                withMapaLock(func() {
                    jogo.StatusMsg = fmt.Sprintf("Entrou no portal! Teletransportado para (%d, %d)", destinoX, destinoY)
                })
                fmt.Printf("Portal ativado: (%d, %d) -> (%d, %d)\n", newX, newY, destinoX, destinoY)
                newX = destinoX
                newY = destinoY
                acaoServidor = "update_position"
                detalheServidor = fmt.Sprintf("X:%d,Y:%d", newX, newY)
                
            } else if elemento == armadilha.Elemento.simbolo {
                // LÓGICA DE ARMADILHA: Penalidade de vida
                jogo.Vidas-- 

                withMapaLock(func() {
                    jogo.StatusMsg = fmt.Sprintf("Caiu em armadilha! Vidas restantes: %d", jogo.Vidas)
                })

				if jogo.Vidas <= 0 {
					jogo.GameOver = true
					jogo.StatusMsg = "GAME OVER — Pressione R para Reiniciar"
					// Não envia RPC de movimento se for Game Over
					return true 
				}
                // O jogador permanece na armadilha na nova posição
                acaoServidor = "update_position" 
                detalheServidor = fmt.Sprintf("X:%d,Y:%d;VIDAS:%d", newX, newY, jogo.Vidas)
                
            } else {
                // Movimento simples
                acaoServidor = "update_position"
                detalheServidor = fmt.Sprintf("X:%d,Y:%d", newX, newY)
            }
            
            // 1c. Atualiza o estado local do jogador com o resultado final da lógica
            jogo.PosX, jogo.PosY = newX, newY 
        } else {
            return true 
        }

        comando = Comando{
            ClientID:       clientID,
            SequenceNumber: sequence,
            Acao:           acaoServidor, 
            Detalhe:        detalheServidor, 
        }
        
    case "interagir":
        comando = Comando{
            ClientID:       clientID,
            SequenceNumber: sequence,
            Acao:           "interact",
        }
    default:
        return true
    }

    // 2. Chamada RPC (com reexecução em caso de falha)
    var resposta Resposta
    for tentativas := 0; tentativas < 3; tentativas++ {
        err := clienteRPC.Call("JogoServer.ExecutarComando", comando, &resposta)
        if err == nil {
            break
        }
        time.Sleep(100 * time.Millisecond)
    }

    if resposta.Sucesso {
        withMapaLock(func() {
            jogo.StatusMsg = resposta.Mensagem
        })
    }
    return true
}