package main

import (
	"fmt"
	"math/rand"
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

// Move o personagem conforme a tecla (WASD)
func personagemMover(tecla rune, jogo *Jogo) {
	// bloqueia movimento após Game Over
	stop := false
	withMapaLock(func() {
		if jogo.GameOver {
			stop = true
		}
	})
	if stop {
		return
	}

	dx, dy := 0, 0
	switch tecla {
	case 'w', 'W':
		dy = -1
	case 'a', 'A':
		dx = -1
	case 's', 'S':
		dy = 1
	case 'd', 'D':
		dx = 1
	default:
		return // ignora outras teclas
	}

	withMapaLock(func() {
		nx, ny := jogo.PosX+dx, jogo.PosY+dy
		// limites do mapa
		if ny < 0 || ny >= len(jogo.Mapa) || nx < 0 || nx >= len(jogo.Mapa[ny]) {
			return
		}

		elem := jogo.Mapa[ny][nx]

		// Guarda: bloqueia
		if elem.simbolo == guarda.Elemento.simbolo {
			jogo.StatusMsg = "O guarda bloqueia o caminho!"
			return
		}

		// Inimigo: bloqueia
		if elem.simbolo == Inimigo.simbolo {
			jogo.StatusMsg = "Um inimigo bloqueia o caminho!"
			return
		}

		// Armadilha: marca Game Over e mostra mensagem normal (sem overlay)
		if elem.simbolo == armadilha.Elemento.simbolo {
			jogo.GameOver = true
			jogo.StatusMsg = "GAME OVER — Pressione R para Reiniciar"
			select {
			case armadilha.ProximidadeJogador <- true:
			default:
			}
			return
		}

		// Portal: teleporte aleatório para célula livre
		if elem.simbolo == portal.Elemento.simbolo {
			// notifica uso do portal (reinicia timeout)
			select {
			case portal.Teletransportar <- true:
			default:
			}

			// coleta destinos livres (não tangíveis), exceto a própria célula atual
			candidatos := make([][2]int, 0, 256)
			for y := 0; y < len(jogo.Mapa); y++ {
				for x := 0; x < len(jogo.Mapa[y]); x++ {
					if !jogo.Mapa[y][x].tangivel && !(x == jogo.PosX && y == jogo.PosY) {
						candidatos = append(candidatos, [2]int{x, y})
					}
				}
			}
			if len(candidatos) == 0 {
				jogo.StatusMsg = "Portal falhou: sem destino livre."
				return
			}

			rand.Seed(time.Now().UnixNano())
			pick := candidatos[rand.Intn(len(candidatos))]
			tx, ty := pick[0], pick[1]

			// Teleporta: restaura célula atual, atualiza posição/UltimoVisitado
			jogo.Mapa[jogo.PosY][jogo.PosX] = jogo.UltimoVisitado
			jogo.PosX, jogo.PosY = tx, ty
			jogo.UltimoVisitado = jogo.Mapa[jogo.PosY][jogo.PosX]

			jogo.StatusMsg = fmt.Sprintf("Você entrou no portal e foi para (%d, %d).", tx, ty)
			return
		}

		// Movimento normal se não for tangível
		if elem.tangivel {
			return
		}
		jogoMoverPersonagem(jogo, dx, dy)
	})
}

// Define o que ocorre quando o jogador pressiona a tecla de interação
func personagemInteragir(jogo *Jogo) {
	jogo.StatusMsg = fmt.Sprintf("Interagindo em (%d, %d)", jogo.PosX, jogo.PosY)
}

// Processa o evento do teclado e executa a ação correspondente
func personagemExecutarAcao(ev EventoTeclado, jogo *Jogo) bool {
	// Reiniciar com 'R' quando em Game Over
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

	switch ev.Tipo {
	case "sair":
		return false
	case "interagir":
		personagemInteragir(jogo)
	case "mover":
		personagemMover(ev.Tecla, jogo)
	}

	// Mantém rodando mesmo em Game Over; espera 'R' ou 'ESC'
	return true
}
