// jogo.go - Funções para manipular os elementos do jogo, como carregar o mapa e mover o personagem
package main

import (
	"bufio"
	"math/rand" // novo
	"os"
	"time"
)

// ------------------ TIPOS BÁSICOS ------------------

// Elemento representa qualquer objeto do mapa (parede, personagem, vegetação, etc)
type Elemento struct {
	simbolo  rune
	cor      Cor
	corFundo Cor
	tangivel bool // Indica se o elemento bloqueia passagem
}

type Guarda struct {
	Elemento
	X, Y             int
	Perseguir        chan bool
	PararPerseguicao chan bool
}

type Portal struct {
	Elemento
	X, Y                int
	Teletransportar     chan bool
	PararTeletransporte chan bool
	Ativo               bool // indica se o portal está aberto no mapa
}

type Armadilha struct {
	Elemento
	X, Y               int
	ProximidadeJogador chan bool
	ProximidadeOutro   chan bool
	PararArmadilha     chan bool
}

// Jogo contém o estado atual do jogo
type Jogo struct {
	Mapa           [][]Elemento // grade 2D representando o mapa
	PosX, PosY     int          // posição atual do personagem
	UltimoVisitado Elemento     // elemento que estava na posição do personagem antes de mover
	StatusMsg      string       // mensagem para a barra de status
	GameOver       bool         // indica que o jogo terminou
}

// ------------------ ELEMENTOS VISUAIS ------------------
var (
	Personagem = Elemento{'☺', CorCinzaEscuro, CorPadrao, true}
	Inimigo    = Elemento{'☠', CorVermelho, CorPadrao, true}
	Parede     = Elemento{'▤', CorParede, CorFundoParede, true}
	Vegetacao  = Elemento{'♣', CorVerde, CorPadrao, false}
	Vazio      = Elemento{' ', CorPadrao, CorPadrao, false}

	// Elementos autônomos
	guarda = &Guarda{
		Elemento:         Elemento{'G', CorAmarelo, CorPadrao, true},
		Perseguir:        make(chan bool),
		PararPerseguicao: make(chan bool),
	}

	portal = &Portal{
		Elemento:            Elemento{'P', CorCiano, CorPadrao, false},
		Teletransportar:     make(chan bool),
		PararTeletransporte: make(chan bool),
	}

	armadilha = &Armadilha{
		Elemento:           Elemento{'A', CorVermelho, CorPadrao, false},
		ProximidadeJogador: make(chan bool),
		ProximidadeOutro:   make(chan bool),
		PararArmadilha:     make(chan bool),
	}
)

// ------------------ CANAL DE LOCK ------------------
var mapaLock = make(chan struct{}, 1)

func withMapaLock(f func()) {
	mapaLock <- struct{}{}
	defer func() { <-mapaLock }()
	f()
}

// ------------------ FUNÇÕES DO JOGO ------------------

// Cria e retorna uma nova instância do jogo
func jogoNovo() Jogo {
	return Jogo{UltimoVisitado: Vazio}
}

// Lê um arquivo texto linha por linha e constrói o mapa do jogo
func jogoCarregarMapa(nome string, jogo *Jogo) error {
	arq, err := os.Open(nome)
	if err != nil {
		return err
	}
	defer arq.Close()

	scanner := bufio.NewScanner(arq)
	y := 0
	for scanner.Scan() {
		linha := scanner.Text()
		var linhaElems []Elemento
		for x, ch := range linha {
			var e Elemento // evita atribuição inicial nunca usada

			switch ch {
			case Parede.simbolo:
				e = Parede
			case Vegetacao.simbolo:
				e = Vegetacao
			case Inimigo.simbolo:
				e = Inimigo
			case Personagem.simbolo:
				// spawn do jogador vem do arquivo, mas a célula fica Vazio
				jogo.PosX, jogo.PosY = x, y
				jogo.UltimoVisitado = Vazio
				e = Vazio
			case 'P', 'G', 'A':
				// elementos autônomos são gerenciados por goroutines
				e = Vazio
			case ' ':
				e = Vazio
			default:
				e = Vazio
			}

			linhaElems = append(linhaElems, e)
		}
		jogo.Mapa = append(jogo.Mapa, linhaElems)
		y++
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
} // ← FECHA jogoCarregarMapa

// Verifica se o personagem pode se mover para a posição (x, y)
func jogoPodeMoverPara(jogo *Jogo, x, y int) bool {
	if y < 0 || y >= len(jogo.Mapa) {
		return false
	}
	if x < 0 || x >= len(jogo.Mapa[y]) {
		return false
	}
	if jogo.Mapa[y][x].tangivel {
		return false
	}
	return true
}

// Move um elemento para a nova posição
func jogoMoverElemento(jogo *Jogo, x, y, dx, dy int) {
	nx, ny := x+dx, y+dy
	// limites
	if ny < 0 || ny >= len(jogo.Mapa) || nx < 0 || nx >= len(jogo.Mapa[ny]) {
		return
	}
	// bloqueio por tangibilidade
	if jogo.Mapa[ny][nx].tangivel {
		return
	}
	// para NPCs, apenas troca as células
	jogoTrocar(jogo, x, y, nx, ny)
}

// ------------------ GOROUTINES AUTÔNOMAS ------------------

func iniciarElementos(jogo *Jogo) {
	go comportamentoGuarda(guarda, jogo)
	go comportamentoPortal(portal, jogo)
	go comportamentoArmadilha(armadilha, jogo)
}

// Troca duas células do mapa (para NPCs)
func jogoTrocar(jogo *Jogo, x, y, nx, ny int) {
	jogo.Mapa[y][x], jogo.Mapa[ny][nx] = jogo.Mapa[ny][nx], jogo.Mapa[y][x]
}

func comportamentoGuarda(guarda *Guarda, jogo *Jogo) {
	withMapaLock(func() {
		guarda.X, guarda.Y = 2, 2
		if guarda.Y < 0 || guarda.Y >= len(jogo.Mapa) ||
			guarda.X < 0 || guarda.X >= len(jogo.Mapa[guarda.Y]) ||
			jogo.Mapa[guarda.Y][guarda.X].tangivel ||
			(guarda.X == jogo.PosX && guarda.Y == jogo.PosY) {
			found := false
			for y := 0; y < len(jogo.Mapa) && !found; y++ {
				for x := 0; x < len(jogo.Mapa[y]); x++ {
					if !jogo.Mapa[y][x].tangivel && !(x == jogo.PosX && y == jogo.PosY) {
						guarda.X, guarda.Y = x, y
						found = true
						break
					}
				}
			}
		}
		jogo.Mapa[guarda.Y][guarda.X] = guarda.Elemento
	})

	rand.Seed(time.Now().UnixNano())

	for {
		stop := false
		withMapaLock(func() {
			if jogo.GameOver {
				stop = true
			}
		})
		if stop {
			return
		}

		moved := false

		select {
		case <-guarda.Perseguir:
			withMapaLock(func() {
				dx, dy := 0, 0
				if jogo.PosX > guarda.X {
					dx = 1
				} else if jogo.PosX < guarda.X {
					dx = -1
				}
				if jogo.PosY > guarda.Y {
					dy = 1
				} else if jogo.PosY < guarda.Y {
					dy = -1
				}
				nx, ny := guarda.X+dx, guarda.Y+dy
				// Verifica se a nova posição é válida E não é o portal
				if !(nx == jogo.PosX && ny == jogo.PosY) &&
					ny >= 0 && ny < len(jogo.Mapa) && nx >= 0 && nx < len(jogo.Mapa[ny]) &&
					!jogo.Mapa[ny][nx].tangivel && jogo.Mapa[ny][nx].simbolo != portal.Elemento.simbolo {
					jogoTrocar(jogo, guarda.X, guarda.Y, nx, ny)
					guarda.X, guarda.Y = nx, ny
					moved = true
				}
			})
		case <-guarda.PararPerseguicao:
			// sem movimento
		default:
			dx := rand.Intn(3) - 1
			dy := rand.Intn(3) - 1
			withMapaLock(func() {
				nx, ny := guarda.X+dx, guarda.Y+dy
				// Verifica se a nova posição é válida E não é o portal
				if !(nx == jogo.PosX && ny == jogo.PosY) &&
					ny >= 0 && ny < len(jogo.Mapa) && nx >= 0 && nx < len(jogo.Mapa[ny]) &&
					!jogo.Mapa[ny][nx].tangivel && jogo.Mapa[ny][nx].simbolo != portal.Elemento.simbolo {
					jogoTrocar(jogo, guarda.X, guarda.Y, nx, ny)
					guarda.X, guarda.Y = nx, ny
					moved = true
				}
			})
		}

		if moved {
			time.Sleep(300 * time.Millisecond)
		} else {
			time.Sleep(120 * time.Millisecond)
		}
	}
}

// PORTAL: abre e fecha com timeout
func comportamentoPortal(portal *Portal, jogo *Jogo) {
	// Funções auxiliares
	abrir := func() {
		withMapaLock(func() {
			portal.Ativo = true
			if len(jogo.Mapa) > portal.Y && len(jogo.Mapa[portal.Y]) > portal.X {
				jogo.Mapa[portal.Y][portal.X] = portal.Elemento
			}
			jogo.StatusMsg = "Um portal apareceu."
		})
	}
	fechar := func(msg string) {
		withMapaLock(func() {
			portal.Ativo = false
			if len(jogo.Mapa) > portal.Y && len(jogo.Mapa[portal.Y]) > portal.X {
				jogo.Mapa[portal.Y][portal.X] = Vazio
			}
			jogo.StatusMsg = msg
		})
	}

	// Posição fixa de exemplo
	withMapaLock(func() { portal.X, portal.Y = 4, 4 })
	abrir()

	// Timer de inatividade (5s enquanto aberto)
	inatividade := time.NewTimer(5 * time.Second)
	resetar := func() {
		if !inatividade.Stop() {
			select {
			case <-inatividade.C:
			default:
			}
		}
		inatividade.Reset(5 * time.Second)
	}

	for {
		// encerra se Game Over
		stop := false
		withMapaLock(func() {
			if jogo.GameOver {
				stop = true
			}
		})
		if stop {
			return
		}

		if portal.Ativo {
			select {
			case <-portal.Teletransportar:
				withMapaLock(func() { jogo.StatusMsg = "Você entrou no portal!" })
				resetar()
			case <-portal.PararTeletransporte:
				fechar("O portal fechou!")
				time.Sleep(8 * time.Second)
				abrir()
				resetar()
			case <-inatividade.C:
				fechar("O portal sumiu por inatividade.")
				time.Sleep(8 * time.Second)
				abrir()
				resetar()
			}
		} else {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func jogoReiniciar(jogo *Jogo) error {
	// sinaliza saída para as goroutines atuais
	jogo.GameOver = true
	time.Sleep(50 * time.Millisecond)

	// recria canais/elementos para novo ciclo
	guarda = &Guarda{
		Elemento:         Elemento{'G', CorAmarelo, CorPadrao, true},
		Perseguir:        make(chan bool),
		PararPerseguicao: make(chan bool),
	}
	portal = &Portal{
		Elemento:            Elemento{'P', CorCiano, CorPadrao, false},
		Teletransportar:     make(chan bool),
		PararTeletransporte: make(chan bool),
	}
	armadilha = &Armadilha{
		Elemento:           Elemento{'A', CorVermelho, CorPadrao, false},
		ProximidadeJogador: make(chan bool),
		ProximidadeOutro:   make(chan bool),
		PararArmadilha:     make(chan bool),
	}

	// recarrega o mapa
	jogo.Mapa = nil
	jogo.UltimoVisitado = Vazio
	if err := jogoCarregarMapa("mapa.txt", jogo); err != nil {
		return err
	}

	jogo.GameOver = false
	jogo.StatusMsg = "Jogo reiniciado."
	iniciarElementos(jogo)
	return nil
}

// ARMADILHA: reage a proximidade
func comportamentoArmadilha(armadilha *Armadilha, jogo *Jogo) {
	withMapaLock(func() {
		armadilha.X, armadilha.Y = 6, 6
		if len(jogo.Mapa) > armadilha.Y && len(jogo.Mapa[armadilha.Y]) > armadilha.X {
			jogo.Mapa[armadilha.Y][armadilha.X] = armadilha.Elemento
		}
	})

	for {
		// encerra se Game Over
		stop := false
		withMapaLock(func() {
			if jogo.GameOver {
				stop = true
			}
		})
		if stop {
			return
		}

		select {
		case <-armadilha.ProximidadeJogador:
			withMapaLock(func() { jogo.StatusMsg = "Você caiu em uma armadilha!" })
		case <-armadilha.ProximidadeOutro:
			withMapaLock(func() { jogo.StatusMsg = "Outro elemento acionou a armadilha!" })
		case <-armadilha.PararArmadilha:
			withMapaLock(func() { jogo.StatusMsg = "A armadilha foi desativada." })
			return
		default:
			time.Sleep(time.Second)
		}
	}
}
