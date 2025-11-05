// jogo.go - Funções para manipular os elementos do jogo, como carregar o mapa e mover o personagem
package main

import (
    "bufio"
    "math/rand"
    "fmt"
    "net/rpc"
    "os"
    "time"
)

// ------------------ TIPOS BÁSICOS ------------------

// Elemento representa qualquer objeto do mapa (parede, personagem, vegetação, etc)
type Elemento struct {
    simbolo  rune
    cor      Cor
    corFundo Cor
    tangivel bool
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
    Ativo               bool 
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
    Mapa           [][]Elemento 
    MapaStatic     [][]Elemento // 💡 NOVO: Cópia do mapa estático para redesenho
    PosX, PosY     int          
    UltimoVisitado Elemento   
    StatusMsg      string      
    GameOver       bool 
    Vidas          int
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

// ---------------- VARIÁVEIS GLOBAIS MULTIPLAYER ------------------
var (
    clienteRPC *rpc.Client 
    clientID   string     
    sequence   int     
)
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
            var e Elemento 

            switch ch {
            case Parede.simbolo:
                e = Parede
            case Vegetacao.simbolo:
                e = Vegetacao
            case Inimigo.simbolo:
                e = Inimigo
            case Personagem.simbolo:
                jogo.PosX, jogo.PosY = x, y
                jogo.UltimoVisitado = Vazio
                e = Vazio
            case 'P':
                e = portal.Elemento 
            case 'G':
                e = Vazio
            case 'A':  
                e = armadilha.Elemento
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
    
    // 💡 NOVO: Inicializa MapaStatic (cópia profunda)
    jogo.MapaStatic = make([][]Elemento, len(jogo.Mapa))
    for i, row := range jogo.Mapa {
        jogo.MapaStatic[i] = make([]Elemento, len(row))
        copy(jogo.MapaStatic[i], row)
    }

    if err := scanner.Err(); err != nil {
        return err
    }
    return nil
}

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
    if ny < 0 || ny >= len(jogo.Mapa) || nx < 0 || nx >= len(jogo.Mapa[ny]) {
        return
    }
    if jogo.Mapa[ny][nx].tangivel {
        return
    }
    jogoTrocar(jogo, x, y, nx, ny)
}

// ------------------ GOROUTINES AUTÔNOMAS ------------------

func iniciarElementos(jogo *Jogo) {
    go comportamentoGuarda(guarda, jogo)
    go comportamentoPortal(portal, jogo)
    go comportamentoArmadilha(armadilha, jogo)
    go loopAtualizacaoCliente(jogo, clienteRPC, clientID)
}

// Troca duas células do mapa (para NPCs)
func jogoTrocar(jogo *Jogo, x, y, nx, ny int) {
    jogo.Mapa[y][x], jogo.Mapa[ny][nx] = jogo.Mapa[ny][nx], jogo.Mapa[y][x]
}

func comportamentoGuarda(guarda *Guarda, jogo *Jogo) {

    guarda.Elemento = Elemento{'G', CorAmarelo, CorPadrao, true}
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
                if !(nx == jogo.PosX && ny == jogo.PosY) &&
                    ny >= 0 && ny < len(jogo.Mapa) && nx >= 0 && nx < len(jogo.Mapa[ny]) &&
                    !jogo.Mapa[ny][nx].tangivel && jogo.Mapa[ny][nx].simbolo != 'O' {
                    jogo.Mapa[guarda.Y][guarda.X] = guarda.Elemento
                    guarda.X, guarda.Y = nx, ny
                    moved = true
                }
            })
        case <-guarda.PararPerseguicao:
        default:
            dx := rand.Intn(3) - 1
            dy := rand.Intn(3) - 1
            withMapaLock(func() {
                nx, ny := guarda.X+dx, guarda.Y+dy
                if !(nx == jogo.PosX && ny == jogo.PosY) &&
                    ny >= 0 && ny < len(jogo.Mapa) && nx >= 0 && nx < len(jogo.Mapa[ny]) &&
                    !jogo.Mapa[ny][nx].tangivel && jogo.Mapa[ny][nx].simbolo != 'O' {
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

func jogoReiniciar(jogo *Jogo) error {
    jogo.GameOver = true
    time.Sleep(50 * time.Millisecond)

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
    jogo.Mapa = nil
    jogo.MapaStatic = nil // Limpa o mapa estático também
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
// ARMADILHA: reage à proximidade e muda de posição
func comportamentoArmadilha(armadilha *Armadilha, jogo *Jogo) {
    withMapaLock(func() {
        armadilha.X, armadilha.Y = 6, 6
        if armadilha.Y < len(jogo.Mapa) && armadilha.X < len(jogo.Mapa[armadilha.Y]) {
            if jogo.Mapa[armadilha.Y][armadilha.X].simbolo == armadilha.Elemento.simbolo {
                jogo.Mapa[armadilha.Y][armadilha.X] = Vazio
            }
        }
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

        select {
        case <-armadilha.ProximidadeJogador:
            // A mensagem é tratada no cliente (personagem.go)
        case <-armadilha.ProximidadeOutro:
            withMapaLock(func() { jogo.StatusMsg = "Outro elemento acionou a armadilha!" })
        case <-armadilha.PararArmadilha:
            withMapaLock(func() { jogo.StatusMsg = "A armadilha foi desativada." })
            return
        default:
            // MOVE A ARMADILHA ALEATORIAMENTE PELO MAPA
            withMapaLock(func() {
                altura := len(jogo.Mapa)
                if altura == 0 {
                    return
                }
                largura := len(jogo.Mapa[0])

                for tentativas := 0; tentativas < 100; tentativas++ {
                    x := rand.Intn(largura)
                    y := rand.Intn(altura)
                    elem := jogo.Mapa[y][x]
                    if !elem.tangivel && elem.simbolo == ' ' {
                        // Limpa posição anterior
                        if armadilha.Y < len(jogo.Mapa) && armadilha.X < len(jogo.Mapa[armadilha.Y]) {
                            jogo.Mapa[armadilha.Y][armadilha.X] = Vazio
                        }
                        // Move a armadilha
                        armadilha.X, armadilha.Y = x, y
                        jogo.Mapa[y][x] = armadilha.Elemento
                        jogo.StatusMsg = fmt.Sprintf("⚠️ A armadilha se moveu para (%d, %d)!", x, y)
                        break
                    }
                }
            })

            // Espera 5 segundos antes de mover de novo
            time.Sleep(10 * time.Second)
        }
    }
}

func comportamentoPortal(portal *Portal, jogo *Jogo) {
    // Define posição inicial do portal
    withMapaLock(func() {
        portal.X, portal.Y = 5, 5 // posição inicial arbitrária
        if portal.Y < len(jogo.Mapa) && portal.X < len(jogo.Mapa[portal.Y]) {
            if jogo.Mapa[portal.Y][portal.X].simbolo == portal.Elemento.simbolo {
                jogo.Mapa[portal.Y][portal.X] = Vazio
            }
        }
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

        select {
        case <-portal.PararTeletransporte:
            withMapaLock(func() { jogo.StatusMsg = "Portal desativado." })
            return
        default:
            // Escolhe nova posição aleatória no mapa
            withMapaLock(func() {
                altura := len(jogo.Mapa)
                if altura == 0 {
                    return
                }
                largura := len(jogo.Mapa[0])

                for tentativas := 0; tentativas < 100; tentativas++ {
                    x := rand.Intn(largura)
                    y := rand.Intn(altura)
                    elem := jogo.Mapa[y][x]
                    if !elem.tangivel && elem.simbolo == ' ' {
                        // Limpa posição anterior
                        if portal.Y < len(jogo.Mapa) && portal.X < len(jogo.Mapa[portal.Y]) {
                            jogo.Mapa[portal.Y][portal.X] = Vazio
                        }
                        // Move o portal
                        portal.X, portal.Y = x, y
                        jogo.Mapa[y][x] = portal.Elemento
                        jogo.StatusMsg = fmt.Sprintf("⚡ O portal se moveu para (%d, %d)!", x, y)
                        break
                    }
                }
            })

            // Espera antes de se mover novamente
            time.Sleep(15 * time.Second)
        }
    }
}

// ------------------ FUNÇÕES CLIENTE MULTIPLAYER ------------------
// loopAtualizacaoCliente busca o estado do jogo no servidor periodicamente e atualiza o estado local.
func loopAtualizacaoCliente(jogo *Jogo, clienteRPC *rpc.Client, clientID string) {
    ticker := time.NewTicker(200 * time.Millisecond)
    defer ticker.Stop()

    for range ticker.C {
        stop := false
        withMapaLock(func() {
            if jogo.GameOver {
                stop = true
            }
        })
        if stop {
            return
        }

        comando := Comando{
            ClientID: clientID,
            Acao:     "BuscarEstado",
        }
        var resposta Resposta

        err := clienteRPC.Call("JogoServer.BuscarEstado", comando, &resposta)
        if err != nil {
            // Se falhar, tenta novamente no próximo tick
            continue 
        }

        withMapaLock(func() {
            // 💡 CORREÇÃO DE MENSAGENS: Apenas atualiza a mensagem se não for uma das mensagens padrão do servidor.
            if resposta.Mensagem != "Estado atual enviado." && resposta.Mensagem != "Posição e Vidas atualizadas..." && resposta.Mensagem != "Posição atualizada..." {
                jogo.StatusMsg = resposta.Mensagem
            }
            
            // 1. Limpa todos os outros jogadores da tela antes de redesenhar
            jogoLimparJogadores(jogo)

            // 2. Itera sobre a lista completa de jogadores fornecida pelo servidor
            for outroID, jogadorEstado := range resposta.EstadoAtual.Jogadores {
                
                // 2a. Sincroniza o Estado do Jogador Local
                if outroID == clientID {
                    // O servidor é a fonte da verdade para Vidas/Posição.
                    jogo.Vidas = jogadorEstado.Vidas
                    jogo.PosX = jogadorEstado.X 
                    jogo.PosY = jogadorEstado.Y
                    continue 
                }

                // 2b. Desenha Outros Jogadores
                if jogadorEstado.Y >= 0 && jogadorEstado.Y < len(jogo.Mapa) &&
                    jogadorEstado.X >= 0 && jogadorEstado.X < len(jogo.Mapa[jogadorEstado.Y]) {
                    
                    // Desenha o outro jogador (símbolo ☺)
                    jogo.Mapa[jogadorEstado.Y][jogadorEstado.X] = Elemento{
                        simbolo:  '☺',
                        cor:      CorAzul,
                        corFundo: CorPadrao,
                        tangivel: true,
                    }
                }
            }
            interfaceDesenharJogo(jogo)
        })
    }
}

// 💡 NOVO: Usa MapaStatic para restaurar o elemento de fundo original
func jogoLimparJogadores(jogo *Jogo) {
    for y := 0; y < len(jogo.Mapa); y++ {
        for x := 0; x < len(jogo.Mapa[y]); x++ {
            // Limpa APENAS o símbolo do Outro Jogador '☺'
            if jogo.Mapa[y][x].simbolo == '☺' { 
                
                // Restaura o elemento de fundo (P, A, Vazio, etc.) da cópia estática
                if y < len(jogo.MapaStatic) && x < len(jogo.MapaStatic[y]) {
                    jogo.Mapa[y][x] = jogo.MapaStatic[y][x] 
                } else {
                    jogo.Mapa[y][x] = Vazio // Fallback
                }
            }
            // Limpa o Guarda (G) para que ele seja redesenhado na próxima posição
            if jogo.Mapa[y][x].simbolo == guarda.Elemento.simbolo {
                // Restaura o elemento de fundo (P, A, Vazio, etc.) da cópia estática
                if y < len(jogo.MapaStatic) && x < len(jogo.MapaStatic[y]) {
                    jogo.Mapa[y][x] = jogo.MapaStatic[y][x] 
                } else {
                    jogo.Mapa[y][x] = Vazio // Fallback
                }
            }
        }
    }
}

func jogoEncontrarSaida(mapa [][]Elemento, origemX, origemY int) (int, int) {
	rand.Seed(time.Now().UnixNano())

	altura := len(mapa)
	if altura == 0 {
		return origemX, origemY
	}
	largura := len(mapa[0])

	for tentativas := 0; tentativas < 100; tentativas++ {
		x := rand.Intn(largura)
		y := rand.Intn(altura)

		elem := mapa[y][x]

		// A nova posição deve ser "vazia" (não tangível)
		if !elem.tangivel && elem.simbolo == ' ' {
			return x, y
		}
	}

	// Se não encontrar posição segura após várias tentativas, retorna a origem
	return origemX, origemY
}

//  $ go run cliente.go jogo.go Structs.go interface.go personagem.go
// go build -o server_jogo server_jogo.go server.go Structs.go 
// ./server_jogo